package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type gitopsCmd struct {
	Reconcile gitopsReconcileCmd `cmd:"" help:"Trigger a GitOps reconciliation"`
	Deploy    gitopsDeployCmd    `cmd:"" help:"Trigger a GitOps deployment"`
	Prune     gitopsPruneCmd     `cmd:"" help:"Trigger a GitOps prune"`
	Validate  gitopsValidateCmd  `cmd:"" help:"Trigger a GitOps validate"`
}

type gitopsCmdHelper struct {
	args args.GitOpsArgs
	kds  []v1beta1.KluctlDeployment

	client       client.Client
	corev1Client *v1.CoreV1Client

	resultStore results.ResultStore
}

func (g *gitopsCmdHelper) init(ctx context.Context, args args.GitOpsArgs) error {
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: args.Context,
	}
	var err error
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		configOverrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	defaultNs, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}

	g.client, err = client.NewWithWatch(restConfig, client.Options{})
	if err != nil {
		return err
	}
	g.corev1Client, err = v1.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	ns := args.Namespace
	if ns == "" {
		ns = defaultNs
	}

	var opts []client.ListOption
	if args.Name != "" {
		if ns == "" {
			return fmt.Errorf("no namespace specified")
		}
		if args.LabelSelector != "" {
			return fmt.Errorf("either name or label selector must be set, not both")
		}

		var kd v1beta1.KluctlDeployment
		err = g.client.Get(ctx, client.ObjectKey{Name: args.Name, Namespace: ns}, &kd)
		if err != nil {
			return err
		}
		g.kds = append(g.kds, kd)
	} else if args.LabelSelector != "" {
		label, err := metav1.ParseToLabelSelector(args.LabelSelector)
		if err != nil {
			return err
		}
		sel, err := metav1.LabelSelectorAsSelector(label)
		if err != nil {
			return err
		}

		opts = append(opts, client.MatchingLabelsSelector{Selector: sel})
		if ns != "" {
			opts = append(opts, client.InNamespace(ns))
		}

		var l v1beta1.KluctlDeploymentList
		err = g.client.List(ctx, &l, opts...)
		if err != nil {
			return err
		}
		g.kds = append(g.kds, l.Items...)
	} else {
		return fmt.Errorf("either name or label selector must be set")
	}

	g.resultStore, err = buildResultStore(ctx, restConfig, args.CommandResultFlags, false)
	if err != nil {
		return err
	}

	return nil
}

func (g *gitopsCmdHelper) patchAnnotation(ctx context.Context, kd *v1beta1.KluctlDeployment, name string, value string) error {
	s := status.Startf(ctx, "Patching KluctlDeployment %s/%s with annotation %s=%s", kd.Namespace, kd.Name, name, value)
	defer s.Failed()

	patch := client.MergeFrom(kd.DeepCopy())

	a := kd.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[name] = value
	kd.SetAnnotations(a)

	err := g.client.Patch(ctx, kd, patch, client.FieldOwner("kluctl"))
	if err != nil {
		return err
	}

	s.Success()

	return nil
}

func (g *gitopsCmdHelper) waitForRequestToFinish(ctx context.Context, key client.ObjectKey, requestValue string, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult) (*v1beta1.RequestResult, error) {
	s := status.Startf(ctx, "Waiting for controller to start processing the request")
	defer s.Failed()

	sleep := time.Second * 1

	var rr *v1beta1.RequestResult
	for {
		var kd v1beta1.KluctlDeployment
		err := g.client.Get(ctx, key, &kd)
		if err != nil {
			return nil, err
		}

		rr = getRequestResult(&kd.Status)
		if rr == nil {
			time.Sleep(sleep)
			continue
		}

		if rr.RequestValue != requestValue {
			time.Sleep(sleep)
			continue
		}
		break
	}
	s.Success()

	status.Infof(ctx, "Watching logs...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logsCh, err := logs.WatchControllerLogs(ctx, g.corev1Client, "kluctl-system", key, rr.ReconcileId, 60*time.Second, true)
	if err != nil {
		return nil, err
	}

	gh := utils.NewGoHelper(ctx, 0)
	gh.RunE(func() error {
		defer func() {
			// give some extra time for log printing
			time.Sleep(sleep)
			cancel()
		}()

		for {
			var kd v1beta1.KluctlDeployment
			err := g.client.Get(ctx, key, &kd)
			if err != nil {
				return fmt.Errorf("unexpected error while getting KluctlDeployment status: %w", err)
			}

			rr = getRequestResult(&kd.Status)
			if rr == nil {
				return fmt.Errorf("request result is nil")
			}
			if rr.FinishedTime != nil {
				return nil
			}
		}
	})
	gh.Run(func() {
		for l := range logsCh {
			status.Info(ctx, l)
		}
	})
	gh.Wait()

	return rr, gh.ErrorOrNil()
}

func init() {
	utilruntime.Must(v1beta1.AddToScheme(scheme.Scheme))
}
