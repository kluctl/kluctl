package args

import "time"

type ProjectFlags struct {
	ProjectUrl string `group:"project" short:"p" help:"Git url of the kluctl project. If not specified, the current directory will be used instead of a remote Git project"`
	ProjectRef string `group:"project" short:"b" help:"Git ref of the kluctl project. Only used when --project-url was given."`

	ProjectConfig       string `group:"project" short:"c" help:"Location of the .kluctl.yml config file. Defaults to $PROJECT/.kluctl.yml" type:"existingfile"`
	LocalClusters       string `group:"project" help:"Local clusters directory. Overrides the project from .kluctl.yml" type:"existingdir"`
	LocalDeployment     string `group:"project" help:"Local deployment directory. Overrides the project from .kluctl.yml" type:"existingdir"`
	LocalSealedSecrets  string `group:"project" help:"Local sealed-secrets directory. Overrides the project from .kluctl.yml" type:"existingdir"`
	FromArchive         string `group:"project" help:"Load project (.kluctl.yml, cluster, ...) from archive. Given path can either be an archive file or a directory with the extracted contents." type:"existing"`
	FromArchiveMetadata string `group:"project" help:"Specify where to load metadata (targets, ...) from. If not specified, metadata is assumed to be part of the archive." type:"existingfile"`
	OutputMetadata      string `group:"project" help:"Specify the output path for the project metadata to be written to. When used with the 'archive' command, it will also cause the archive to not include the metadata.yaml file." type:"file"`
	Cluster             string `group:"project" help:"Specify/Override cluster"`

	LoadTimeout time.Duration `group:"project" help:"Specify timeout for project loading. This will especially limit the time spent in git operations." default:"1m"`
}

type ArgsFlags struct {
	Arg []string `group:"project" short:"a" help:"Template argument in the form name=value"`
}

type TargetFlags struct {
	Target string `group:"project" short:"t" help:"Target name to run command for. Target must exist in .kluctl.yml."`
}
