package args

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KluctlDeploymentFlags struct {
	KluctlDeployment string `group:"flux" short:"k" help:"Name of the KluctlDeployment to interact with"`
	Namespace        string `group:"flux" short:"n" help:"Namespace where KluctlDeployment is located"`
	WithSource       bool   `group:"flux" help:"--with-source will annotate Source object as well, triggering pulling"`
}

var KluctlDeploymentGVK = schema.GroupVersionKind{
	Group:   "flux.kluctl.io",
	Version: "v1alpha1",
	Kind:    "KluctlDeployment",
}

var GitRepositoryGVK = schema.GroupVersionKind{
	Group:   "source.toolkit.fluxcd.io",
	Version: "v1beta2",
	Kind:    "GitRepository",
}

func (cmd *KluctlDeploymentFlags) VerifyFlags() bool {
	if cmd.KluctlDeployment == "" {
		return false
	}
	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
	return true
}
