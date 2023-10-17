package args

type GitOpsArgs struct {
	CommandResultFlags

	Name          string `group:"misc" help:"Specifies the name of the KluctlDeployment."`
	Namespace     string `group:"misc" short:"n" help:"Specifies the namespace of the KluctlDeployment. If omitted, the current namespace from your kubeconfig is used."`
	LabelSelector string `group:"misc" short:"l" help:"If specified, KluctlDeployments are searched and filtered by this label selector."`

	Context string `group:"misc" help:"Override the context to use."`
}
