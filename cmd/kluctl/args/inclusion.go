package args

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"os"
	"path/filepath"
	"strings"
)

type InclusionFlags struct {
	IncludeTag           []string `group:"inclusion" short:"I" help:"Include deployments with given tag."`
	ExcludeTag           []string `group:"inclusion" short:"E" help:"Exclude deployments with given tag. Exclusion has precedence over inclusion, meaning that explicitly excluded deployments will always be excluded even if an inclusion rule would match the same deployment."`
	IncludeDeploymentDir []string `group:"inclusion" help:"Include deployment dir. The path must be relative to the root deployment project."`
	ExcludeDeploymentDir []string `group:"inclusion" help:"Exclude deployment dir. The path must be relative to the root deployment project. Exclusion has precedence over inclusion, same as in --exclude-tag"`
}

func (args *InclusionFlags) ParseInclusionFromArgs() (*utils.Inclusion, error) {
	inclusion := utils.NewInclusion()
	for _, tag := range args.IncludeTag {
		inclusion.AddInclude("tag", tag)
	}
	for _, tag := range args.ExcludeTag {
		inclusion.AddExclude("tag", tag)
	}
	for _, dir := range args.IncludeDeploymentDir {
		if filepath.IsAbs(dir) {
			return nil, fmt.Errorf("--include-deployment-dir path must be relative")
		}
		inclusion.AddInclude("deploymentItemDir", strings.ReplaceAll(dir, string(os.PathSeparator), "/"))
	}
	for _, dir := range args.ExcludeDeploymentDir {
		if filepath.IsAbs(dir) {
			return nil, fmt.Errorf("--exclude-deployment-dir path must be relative")
		}
		inclusion.AddExclude("deploymentItemDir", strings.ReplaceAll(dir, string(os.PathSeparator), "/"))
	}
	return inclusion, nil
}
