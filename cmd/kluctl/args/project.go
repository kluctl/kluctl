package args

import (
	"github.com/spf13/cobra"
)

var (
	ProjectUrl          string
	ProjectRef          string
	ProjectConfig       string
	LocalClusters       string
	LocalDeployment     string
	LocalSealedSecrets  string
	FromArchive         string
	FromArchiveMetadata string
	Cluster             string

	Args []string

	Target string
)

func AddProjectArgs(cmd *cobra.Command, withDeploymentArgs bool, withArgs bool, withTarget bool) {
	if withDeploymentArgs {
		cmd.Flags().StringVarP(&ProjectUrl, "project-url", "p", "", "Git url of the kluctl project. If not specified, the current directory will be used instead of a remote Git project")
		cmd.Flags().StringVarP(&ProjectRef, "project-ref", "b", "", "Git ref of the kluctl project. Only used when --project-url was given.")

		cmd.Flags().StringVarP(&ProjectConfig, "project-config", "c", "", "Location of the .kluctl.yml config file. Defaults to $PROJECT/.kluctl.yml")
		cmd.Flags().StringVar(&LocalClusters, "local-clusters", "", "Local clusters directory. Overrides the project from .kluctl.yml")
		cmd.Flags().StringVar(&LocalDeployment, "local-deployment", "", "Local deployment directory. Overrides the project from .kluctl.yml")
		cmd.Flags().StringVar(&LocalSealedSecrets, "local-sealed-secrets", "", "Local sealed-secrets directory. Overrides the project from .kluctl.yml")
		cmd.Flags().StringVar(&FromArchive, "from-archive", "", "Load project (.kluctl.yml, cluster, ...) from archive. Given path can either be an archive file or a directory with the extracted contents.")
		cmd.Flags().StringVar(&FromArchiveMetadata, "from-archive-metadata", "", "Specify where to load metadata (targets, ...) from. If not specified, metadata is assumed to be part of the archive.")
		cmd.Flags().StringVar(&Cluster, "cluster", "", "Specify/Override cluster")

		_ = cmd.MarkFlagFilename("project-config")
		_ = cmd.MarkFlagDirname("local-clusters")
		_ = cmd.MarkFlagDirname("local-deployment")
		_ = cmd.MarkFlagDirname("local-sealed-secrets")
		_ = cmd.MarkFlagFilename("from-archive")
		_ = cmd.MarkFlagFilename("from-archive-metadata")
	}

	if withArgs {
		cmd.Flags().StringArrayVarP(&Args, "arg", "a", nil, "Template argument in the form name=value")
	}

	if withTarget {
		cmd.Flags().StringVarP(&Target, "target", "t", "", "Target name to run command for. Target must exist in .kluctl.yml.")
	}
}
