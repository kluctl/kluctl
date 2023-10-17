package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	g.corev1Client = v1.NewForConfigOrDie(restConfig)

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

func init() {
	utilruntime.Must(v1beta1.AddToScheme(scheme.Scheme))
}
