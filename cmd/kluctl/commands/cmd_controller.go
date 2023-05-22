package commands

import (
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/controllers"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"
	"os/user"
	"path/filepath"
	"sigs.k8s.io/cli-utils/pkg/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	setupLog        = ctrl.Log.WithName("setup")
	metricsRecorder = metrics.NewRecorder()
)

func init() {
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)
}

const controllerName = "kluctl-controller"

type controllerCmd struct {
	scheme *runtime.Scheme

	Kubeconfig string `group:"controller" help:"Override the kubeconfig to use."`
	Context    string `group:"controller" help:"Override the context to use."`

	MetricsBindAddress     string `group:"controller" help:"The address the metric endpoint binds to." default:":8080"`
	HealthProbeBindAddress string `group:"controller" help:"The address the probe endpoint binds to." default:":8081"`
	LeaderElect            bool   `group:"controller" help:"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager."`

	DefaultServiceAccount string `group:"controller" help:"Default service account used for impersonation."`
	DryRun                bool   `group:"controller" help:"Run all deployments in dryRun=true mode."`
}

func (cmd *controllerCmd) Help() string {
	return `This command will run the Kluctl Controller. This is usually meant to be run inside a cluster and not from your local machine.
`
}

func (cmd *controllerCmd) initScheme() {
	cmd.scheme = runtime.NewScheme()
	scheme := cmd.scheme

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kluctlv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func (cmd *controllerCmd) Run(ctx context.Context) error {
	cmd.initScheme()

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	restConfig, err := cmd.loadConfig(cmd.Kubeconfig, cmd.Context)
	if err != nil {
		setupLog.Error(err, "unable to load kubeconfig")
		os.Exit(1)
	}

	enabled, err := flowcontrol.IsEnabled(context.Background(), restConfig)
	if err == nil && enabled {
		// A negative QPS and Burst indicates that the client should not have a rate limiter.
		// Ref: https://github.com/kubernetes/kubernetes/blob/v1.24.0/staging/src/k8s.io/client-go/rest/config.go#L354-L364
		restConfig.QPS = -1
		restConfig.Burst = -1
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 cmd.scheme,
		MetricsBindAddress:     cmd.MetricsBindAddress,
		Port:                   9443,
		HealthProbeBindAddress: cmd.HealthProbeBindAddress,
		LeaderElection:         cmd.LeaderElect,
		LeaderElectionID:       "5ab5d0f9.kluctl.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	eventRecorder := mgr.GetEventRecorderFor(controllerName)

	sshPool := &ssh_pool.SshPool{}

	r := controllers.KluctlDeploymentReconciler{
		ControllerName:        controllerName,
		DefaultServiceAccount: cmd.DefaultServiceAccount,
		DryRun:                cmd.DryRun,
		RestConfig:            restConfig,
		Client:                mgr.GetClient(),
		ClientSet:             clientSet,
		Scheme:                mgr.GetScheme(),
		EventRecorder:         eventRecorder,
		MetricsRecorder:       metricsRecorder,
		SshPool:               sshPool,
	}

	if err = r.SetupWithManager(ctx, mgr, controllers.KluctlDeploymentReconcilerOpts{
		HTTPRetry: 9,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", kluctlv1.KluctlDeploymentKind)
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}

// taken from clientcmd
func (cmd *controllerCmd) loadConfig(kubeconfig string, context string) (config *rest.Config, configErr error) {
	// If a flag is specified with the config location, use that
	if len(kubeconfig) > 0 {
		return cmd.loadConfigWithContext("", &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}, context)
	}

	// If the recommended kubeconfig env variable is not specified,
	// try the in-cluster config.
	kubeconfigPath := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	if len(kubeconfigPath) == 0 {
		c, err := rest.InClusterConfig()
		if err == nil {
			return c, nil
		}

		defer func() {
			if configErr != nil {
				log.Error(err, "unable to load in-cluster config")
			}
		}()
	}

	// If the recommended kubeconfig env variable is set, or there
	// is no in-cluster config, try the default recommended locations.
	//
	// NOTE: For default config file locations, upstream only checks
	// $HOME for the user's home directory, but we can also try
	// os/user.HomeDir when $HOME is unset.
	//
	// TODO(jlanford): could this be done upstream?
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if _, ok := os.LookupEnv("HOME"); !ok {
		u, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("could not get current user: %w", err)
		}
		loadingRules.Precedence = append(loadingRules.Precedence, filepath.Join(u.HomeDir, clientcmd.RecommendedHomeDir, clientcmd.RecommendedFileName))
	}

	return cmd.loadConfigWithContext("", loadingRules, context)
}

// taken from clientcmd
func (cmd *controllerCmd) loadConfigWithContext(apiServerURL string, loader clientcmd.ClientConfigLoader, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader,
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: apiServerURL,
			},
			CurrentContext: context,
		}).ClientConfig()
}
