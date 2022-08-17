package args

import "k8s.io/apimachinery/pkg/runtime/schema"

type KluctlDeploymentFlags struct {
	KluctlDeployment string `group:"flux" short:"k" help:"Name of the KluctlDeployment to interact with"`
	Namespace        string `group:"flux" short:"n" help:"Namespace where KluctlDeployment is located"`
	WithSource       bool   `group:"flux" help:"--with-source will annotate Source object as well, triggering pulling"`
}

var KluctlDeployment = schema.GroupVersionResource{
	Group:    "flux.kluctl.io",
	Version:  "v1alpha1",
	Resource: "kluctldeployments",
}

var GitRepository = schema.GroupVersionResource{
	Group:    "source.toolkit.fluxcd.io",
	Version:  "v1beta2",
	Resource: "gitrepositories",
}
