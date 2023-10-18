package args

import "time"

type GitOpsArgs struct {
	CommandResultFlags

	Name          string `group:"misc" help:"Specifies the name of the KluctlDeployment."`
	Namespace     string `group:"misc" short:"n" help:"Specifies the namespace of the KluctlDeployment. If omitted, the current namespace from your kubeconfig is used."`
	LabelSelector string `group:"misc" short:"l" help:"If specified, KluctlDeployments are searched and filtered by this label selector."`

	Context string `group:"misc" help:"Override the context to use."`
}

func (a GitOpsArgs) AnyObjectArgSet() bool {
	return a.Name != "" || a.Namespace != "" || a.LabelSelector != ""
}

type GitOpsLogArgs struct {
	LogSince        time.Duration `group:"misc" help:"Show logs since this time." default:"60s"`
	LogGroupingTime time.Duration `group:"misc" help:"Logs are by default grouped by time passed, meaning that they are printed in batches to make reading them easier. This argument allows to modify the grouping time." default:"1s"`
	LogTime         bool          `group:"misc" help:"If enabled, adds timestamps to log lines"`
}
