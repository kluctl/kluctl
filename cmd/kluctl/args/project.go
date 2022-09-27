package args

import "time"

type ProjectFlags struct {
	ProjectUrl string `group:"project" short:"p" help:"Git url of the kluctl project. If not specified, the current directory will be used instead of a remote Git project"`
	ProjectRef string `group:"project" short:"b" help:"Git ref of the kluctl project. Only used when --project-url was given."`

	ProjectConfig      existingFileType `group:"project" short:"c" help:"Location of the .kluctl.yaml config file. Defaults to $PROJECT/.kluctl.yaml" exts:"yml,yaml"`
	LocalClusters      existingDirType  `group:"project" help:"DEPRECATED. Local clusters directory. Overrides the project from .kluctl.yaml"`
	LocalDeployment    existingDirType  `group:"project" help:"DEPRECATED. Local deployment directory. Overrides the project from .kluctl.yaml"`
	LocalSealedSecrets existingDirType  `group:"project" help:"DEPRECATED. Local sealed-secrets directory. Overrides the project from .kluctl.yaml" `
	OutputMetadata     string           `group:"project" help:"Specify the output path for the project metadata to be written to."`
	Cluster            string           `group:"project" help:"DEPRECATED. Specify/Override cluster"`

	Timeout                time.Duration `group:"project" help:"Specify timeout for all operations, including loading of the project, all external api calls and waiting for readiness." default:"10m"`
	GitCacheUpdateInterval time.Duration `group:"project" help:"Specify the time to wait between git cache updates. Defaults to not wait at all and always updating caches."`
}

type ArgsFlags struct {
	Arg []string `group:"project" short:"a" help:"Passes a template argument in the form of name=value. Nested args can be set with the '-a my.nested.arg=value' syntax. Values are interpreted as yaml values, meaning that 'true' and 'false' will lead to boolean values and numbers will be treated as numbers. Use quotes if you want these to be treated as strings."`
}

type TargetFlags struct {
	Target string `group:"project" short:"t" help:"Target name to run command for. Target must exist in .kluctl.yaml."`
}
