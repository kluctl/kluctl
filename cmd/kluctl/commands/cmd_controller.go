package commands

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/controllers"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
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

	restConfig := flux_utils.GetConfigOrDie(flux_utils.Options{})
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
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
