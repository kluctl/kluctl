package args

import (
	"github.com/spf13/cobra"
	"time"
)

var (
	ForceYes            bool
	DryRun              bool
	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	HookTimeout         time.Duration
	IgnoreTags          bool
	IgnoreLabels        bool
	IgnoreAnnotations   bool
	AbortOnError        bool
	OutputFormat        []string
	Output              []string
	RenderOutputDir     string
)

type EnabledMiscArguments struct {
	Yes             bool
	DryRun          bool
	ForceApply      bool
	ReplaceOnError  bool
	HookTimeout     bool
	IgnoreLabels    bool
	AbortOnError    bool
	OutputFormat    bool
	Output          bool
	RenderOutputDir bool
}

func AddMiscArguments(cmd *cobra.Command, enabledArgs EnabledMiscArguments) {
	if enabledArgs.Yes {
		cmd.Flags().BoolVarP(&ForceYes, "yes", "y", false, "Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.")
	}
	if enabledArgs.DryRun {
		cmd.Flags().BoolVar(&DryRun, "dry-run", false, "Performs all kubernetes API calls in dry-run mode.")
	}
	if enabledArgs.ForceApply {
		cmd.Flags().BoolVar(&ForceApply, "force-apply", false, "Force conflict resolution when applying. See documentation for details")
	}
	if enabledArgs.ReplaceOnError {
		cmd.Flags().BoolVar(&ReplaceOnError, "replace-on-error", false, "When patching an object fails, try to replace it. See documentation for more details.")
		cmd.Flags().BoolVar(&ForceReplaceOnError, "force-replace-on-error", false, "Same as --replace-on-error, but also try to delete and re-create objects. See documentation for more details.")
	}
	if enabledArgs.HookTimeout {
		cmd.Flags().DurationVar(&HookTimeout, "hook-timeout", 5*time.Minute, "Maximum time to wait for hook readiness. The timeout is meant per-hook. Timeouts are in the duration format (1s, 1m, 1h, ...). If not specified, a default timeout of 5m is used.")
	}
	if enabledArgs.IgnoreLabels {
		cmd.Flags().BoolVar(&IgnoreTags, "ignore-tags", false, "Ignores changes in tags when diffing")
		cmd.Flags().BoolVar(&IgnoreLabels, "ignore-labels", false, "Ignores changes in labels when diffing")
		cmd.Flags().BoolVar(&IgnoreAnnotations, "ignore-annotations", false, "Ignores changes in annotations when diffing")
	}
	if enabledArgs.AbortOnError {
		cmd.Flags().BoolVar(&AbortOnError, "abort-on-error", false, "Abort deploying when an error occurs instead of trying the remaining deployments")
	}
	if enabledArgs.OutputFormat {
		cmd.Flags().StringArrayVarP(&OutputFormat, "output", "o", nil, "Specify output format and target file, in the format 'format=path'. Format can either be 'text' or 'yaml'. Can be specified multiple times. The actual format for yaml is currently not documented and subject to change.")
	}
	if enabledArgs.Output {
		cmd.Flags().StringArrayVarP(&Output, "output", "o", nil, "Specify output target file. Can be specified multiple times")
	}
	if enabledArgs.RenderOutputDir {
		cmd.Flags().StringVar(&RenderOutputDir, "render-output-dir", "", "Specifies the target directory to render the project into. If omitted, a temporary directory is used.")
		_ = cmd.MarkFlagDirname("render-output-dir")
	}
}
