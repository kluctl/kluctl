package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

func RegisterFlagCompletionFuncs(cmd interface{}, ccmd *cobra.Command) error {
	v := reflect.ValueOf(cmd).Elem()
	projectFlags := v.FieldByName("ProjectFlags")
	targetFlags := v.FieldByName("TargetFlags")

	if projectFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("cluster", buildClusterCompletionFunc(projectFlags.Addr().Interface().(*args.ProjectFlags)))
	}

	if projectFlags.IsValid() && targetFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("target", buildTargetCompletionFunc(projectFlags.Addr().Interface().(*args.ProjectFlags)))
	}

	return nil
}

func withProjectForCompletion(projectArgs *args.ProjectFlags, cb func(p *kluctl_project.KluctlProjectContext) error) error {
	// let's not update git caches too often
	projectArgs.GitCacheUpdateInterval = time.Second * 60
	return withKluctlProjectFromArgs(*projectArgs, func(p *kluctl_project.KluctlProjectContext) error {
		return cb(p)
	})
}

func buildClusterCompletionFunc(projectArgs *args.ProjectFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var ret []string
		err := withProjectForCompletion(projectArgs, func(p *kluctl_project.KluctlProjectContext) error {
			dents, err := os.ReadDir(p.ClustersDir)
			if err != nil {
				return err
			}
			for _, de := range dents {
				var config types.ClusterConfig
				err = yaml.ReadYamlFile(filepath.Join(p.ClustersDir, de.Name()), &config)
				if err != nil {
					continue
				}
				if config.Cluster.Name != "" {
					ret = append(ret, config.Cluster.Name)
				}
			}
			return nil
		})
		if err != nil {
			log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
		return ret, cobra.ShellCompDirectiveDefault
	}
}

func buildTargetCompletionFunc(projectArgs *args.ProjectFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var ret []string
		err := withProjectForCompletion(projectArgs, func(p *kluctl_project.KluctlProjectContext) error {
			for _, t := range p.DynamicTargets {
				ret = append(ret, t.Target.Name)
			}
			return nil
		})
		if err != nil {
			log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}
		return ret, cobra.ShellCompDirectiveDefault
	}
}
