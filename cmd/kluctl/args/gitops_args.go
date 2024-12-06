package args

import (
	"time"
)

type GitOpsArgs struct {
	ProjectDir
	CommandResultReadOnlyFlags

	Name          string `group:"gitops" help:"Specifies the name of the KluctlDeployment."`
	Namespace     string `group:"gitops" short:"n" help:"Specifies the namespace of the KluctlDeployment. If omitted, the current namespace from your kubeconfig is used."`
	LabelSelector string `group:"gitops" short:"l" help:"If specified, KluctlDeployments are searched and filtered by this label selector."`

	Kubeconfig          ExistingFileType `group:"gitops" help:"Overrides the kubeconfig to use."`
	Context             string           `group:"gitops" help:"Override the context to use."`
	ControllerNamespace string           `group:"gitops" help:"The namespace where the controller runs in." default:"kluctl-system"`

	LocalSourceOverridePort int `group:"gitops" help:"Specifies the local port to which the source-override client should connect to when running the controller locally." default:"0"`
}

type GitOpsLogArgs struct {
	LogSince        time.Duration `group:"logs" help:"Show logs since this time." default:"60s"`
	LogGroupingTime time.Duration `group:"logs" help:"Logs are by default grouped by time passed, meaning that they are printed in batches to make reading them easier. This argument allows to modify the grouping time." default:"1s"`
	LogTime         bool          `group:"logs" help:"If enabled, adds timestamps to log lines"`
}

type GitOpsOverridableArgs struct {
	SourceOverrides
	TargetFlagsBase
	ArgsFlags
	ImageFlags
	InclusionFlags
	DryRunFlags
	ForceApplyFlags
	ReplaceOnErrorFlags
	AbortOnErrorFlags

	TargetContext string `group:"override" help:"Overrides the context name specified in the target. If the selected target does not specify a context or the no-name target is used, --context will override the currently active context."`
}
