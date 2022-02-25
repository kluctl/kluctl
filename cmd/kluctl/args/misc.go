package args

import (
	"time"
)

type YesFlags struct {
	Yes bool `group:"misc" short:"y" help:"Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'."`
}

type DryRunFlags struct {
	DryRun bool `group:"misc" help:"Performs all kubernetes API calls in dry-run mode."`
}

type ForceApplyFlags struct {
	ForceApply bool `group:"misc" help:"Force conflict resolution when applying. See documentation for details"`
}

type ReplaceOnErrorFlags struct {
	ReplaceOnError      bool `group:"misc" help:"When patching an object fails, try to replace it. See documentation for more details."`
	ForceReplaceOnError bool `group:"misc" help:"Same as --replace-on-error, but also try to delete and re-create objects. See documentation for more details."`
}

type HookFlags struct {
	HookTimeout time.Duration `group:"misc" help:"Maximum time to wait for hook readiness. The timeout is meant per-hook. Timeouts are in the duration format (1s, 1m, 1h, ...). If not specified, a default timeout of 5m is used." default:"5m"`
}

type IgnoreFlags struct {
	IgnoreTags        bool `group:"misc" help:"Ignores changes in tags when diffing"`
	IgnoreLabels      bool `group:"misc" help:"Ignores changes in labels when diffing"`
	IgnoreAnnotations bool `group:"misc" help:"Ignores changes in annotations when diffing"`
}

type AbortOnErrorFlags struct {
	AbortOnError bool `group:"misc" help:"Abort deploying when an error occurs instead of trying the remaining deployments"`
}

type OutputFormatFlags struct {
	OutputFormat []string `group:"misc" short:"o" help:"Specify output format and target file, in the format 'format=path'. Format can either be 'text' or 'yaml'. Can be specified multiple times. The actual format for yaml is currently not documented and subject to change."`
}

type OutputFlags struct {
	Output []string `group:"misc" short:"o" help:"Specify output target file. Can be specified multiple times"`
}

type RenderOutputDirFlags struct {
	RenderOutputDir string `group:"misc" help:"Specifies the target directory to render the project into. If omitted, a temporary directory is used." type:"path"`
}
