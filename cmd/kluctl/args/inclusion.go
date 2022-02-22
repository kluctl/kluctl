package args

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var (
	IncludeTags           []string
	ExcludeTags           []string
	IncludeDeploymentDirs []string
	ExcludeDeploymentDirs []string
)

func AddInclusionArgs(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&IncludeTags, "include-tags", "I", nil, "Include deployments with given tag.")
	cmd.Flags().StringArrayVarP(&ExcludeTags, "exclude-tags", "E", nil, "Exclude deployments with given tag. Exclusion has precedence over inclusion, meaning that explicitly excluded deployments will always be excluded even if an inclusion rule would match the same deployment.")
	cmd.Flags().StringArrayVar(&IncludeDeploymentDirs, "include-deployment-dir", nil, "Include deployment dir. The path must be relative to the root deployment project.")
	cmd.Flags().StringArrayVar(&ExcludeDeploymentDirs, "exclude-deployment-dir", nil, "Exclude deployment dir. The path must be relative to the root deployment project. Exclusion has precedence over inclusion, same as in --exclude-tag")
}

func ParseInclusionFromArgs() (*utils.Inclusion, error) {
	inclusion := utils.NewInclusion()
	for _, tag := range IncludeTags {
		inclusion.AddInclude("tag", tag)
	}
	for _, tag := range ExcludeTags {
		inclusion.AddExclude("tag", tag)
	}
	for _, dir := range IncludeDeploymentDirs {
		if filepath.IsAbs(dir) {
			return nil, fmt.Errorf("--include-deployment-dir path must be relative")
		}
		inclusion.AddInclude("deploymentItemDir", strings.ReplaceAll(dir, string(os.PathSeparator), "/"))
	}
	for _, dir := range ExcludeDeploymentDirs {
		if filepath.IsAbs(dir) {
			return nil, fmt.Errorf("--exclude-deployment-dir path must be relative")
		}
		inclusion.AddExclude("deploymentItemDir", strings.ReplaceAll(dir, string(os.PathSeparator), "/"))
	}
	return inclusion, nil
}
