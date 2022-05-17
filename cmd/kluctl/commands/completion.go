package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

func RegisterFlagCompletionFuncs(cmdStruct interface{}, ccmd *cobra.Command) error {
	v := reflect.ValueOf(cmdStruct).Elem()
	projectFlags := v.FieldByName("ProjectFlags")
	targetFlags := v.FieldByName("TargetFlags")
	inclusionFlags := v.FieldByName("InclusionFlags")
	imageFlags := v.FieldByName("ImageFlags")

	if projectFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("cluster", buildClusterCompletionFunc(projectFlags.Addr().Interface().(*args.ProjectFlags)))
	}

	if projectFlags.IsValid() && targetFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("target", buildTargetCompletionFunc(projectFlags.Addr().Interface().(*args.ProjectFlags)))
	}

	if projectFlags.IsValid() && inclusionFlags.IsValid() {
		tagsFunc := buildInclusionCompletionFunc(cmdStruct, false)
		dirsFunc := buildInclusionCompletionFunc(cmdStruct, true)
		_ = ccmd.RegisterFlagCompletionFunc("include-tag", tagsFunc)
		_ = ccmd.RegisterFlagCompletionFunc("exclude-tag", tagsFunc)
		_ = ccmd.RegisterFlagCompletionFunc("include-deployment-dir", dirsFunc)
		_ = ccmd.RegisterFlagCompletionFunc("exclude-deployment-dir", dirsFunc)
	}

	if imageFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("fixed-image", buildImagesCompletionFunc(cmdStruct))
	}

	return nil
}

func withProjectForCompletion(projectArgs *args.ProjectFlags, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
	// let's not update git caches too often
	projectArgs.GitCacheUpdateInterval = time.Second * 60
	return withKluctlProjectFromArgs(*projectArgs, false, true, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		return cb(ctx, p)
	})
}

func buildClusterCompletionFunc(projectArgs *args.ProjectFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var ret []string
		err := withProjectForCompletion(projectArgs, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
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
			status.Error(cliCtx, err.Error())
			return nil, cobra.ShellCompDirectiveError
		}
		return ret, cobra.ShellCompDirectiveDefault
	}
}

func buildTargetCompletionFunc(projectArgs *args.ProjectFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var ret []string
		err := withProjectForCompletion(projectArgs, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
			for _, t := range p.DynamicTargets {
				ret = append(ret, t.Target.Name)
			}
			return nil
		})
		if err != nil {
			status.Error(cliCtx, err.Error())
			return nil, cobra.ShellCompDirectiveError
		}
		return ret, cobra.ShellCompDirectiveDefault
	}
}

func buildAutocompleteProjectTargetCommandArgs(cmdStruct interface{}) projectTargetCommandArgs {
	ptArgs := projectTargetCommandArgs{}

	cmdV := reflect.ValueOf(cmdStruct).Elem()
	if cmdV.FieldByName("ProjectFlags").IsValid() {
		ptArgs.projectFlags = cmdV.FieldByName("ProjectFlags").Interface().(args.ProjectFlags)
	}
	if cmdV.FieldByName("TargetFlags").IsValid() {
		ptArgs.targetFlags = cmdV.FieldByName("TargetFlags").Interface().(args.TargetFlags)
	}
	if cmdV.FieldByName("ArgsFlags").IsValid() {
		ptArgs.argsFlags = cmdV.FieldByName("ArgsFlags").Interface().(args.ArgsFlags)
	}
	if cmdV.FieldByName("ImageFlags").IsValid() {
		ptArgs.imageFlags = cmdV.FieldByName("ImageFlags").Interface().(args.ImageFlags)
	}
	if cmdV.FieldByName("InclusionFlags").IsValid() {
		ptArgs.inclusionFlags = cmdV.FieldByName("InclusionFlags").Interface().(args.InclusionFlags)
	}

	ptArgs.forCompletion = true
	return ptArgs
}

func buildInclusionCompletionFunc(cmdStruct interface{}, forDirs bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ptArgs := buildAutocompleteProjectTargetCommandArgs(cmdStruct)

		var tags utils.OrderedMap
		var deploymentItemDirs utils.OrderedMap
		var mutex sync.Mutex

		err := withProjectForCompletion(&ptArgs.projectFlags, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
			var targets []string
			if ptArgs.targetFlags.Target == "" {
				for _, t := range p.DynamicTargets {
					targets = append(targets, t.Target.Name)
				}
			} else {
				targets = append(targets, ptArgs.targetFlags.Target)
			}

			var wg sync.WaitGroup
			for _, t := range targets {
				ptArgs := ptArgs
				ptArgs.targetFlags.Target = t
				wg.Add(1)
				go func() {
					_ = withProjectTargetCommandContext(ctx, ptArgs, p, func(ctx *commandCtx) error {
						mutex.Lock()
						defer mutex.Unlock()
						for _, di := range ctx.targetCtx.DeploymentCollection.Deployments {
							tags.Merge(di.Tags)
							deploymentItemDirs.Set(di.RelToRootItemDir, true)
						}
						return nil
					})
					wg.Done()
				}()
			}
			wg.Wait()
			return nil
		})
		if err != nil {
			status.Error(cliCtx, err.Error())
			return nil, cobra.ShellCompDirectiveError
		}
		if forDirs {
			return deploymentItemDirs.ListKeys(), cobra.ShellCompDirectiveDefault
		} else {
			return tags.ListKeys(), cobra.ShellCompDirectiveDefault
		}
	}
}

func buildImagesCompletionFunc(cmdStruct interface{}) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ptArgs := buildAutocompleteProjectTargetCommandArgs(cmdStruct)

		if strings.Index(toComplete, "=") != -1 {
			return nil, cobra.ShellCompDirectiveDefault
		}

		var images utils.OrderedMap
		var mutex sync.Mutex

		err := withProjectForCompletion(&ptArgs.projectFlags, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
			var targets []string
			if ptArgs.targetFlags.Target == "" {
				for _, t := range p.DynamicTargets {
					targets = append(targets, t.Target.Name)
				}
			} else {
				targets = append(targets, ptArgs.targetFlags.Target)
			}

			var wg sync.WaitGroup
			for _, t := range targets {
				ptArgs := ptArgs
				ptArgs.targetFlags.Target = t
				wg.Add(1)
				go func() {
					_ = withProjectTargetCommandContext(ctx, ptArgs, p, func(ctx *commandCtx) error {
						err := ctx.targetCtx.DeploymentCollection.Prepare(nil)
						if err != nil {
							status.Error(cliCtx, err.Error())
						}

						mutex.Lock()
						defer mutex.Unlock()
						for _, si := range ctx.images.SeenImages(false) {
							str := si.Image
							if si.Namespace != nil {
								str += ":" + *si.Namespace
							}
							if si.Deployment != nil {
								str += ":" + *si.Deployment
							}
							if si.Container != nil {
								str += ":" + *si.Container
							}
							images.Set(str, true)
						}
						return nil
					})
					wg.Done()
				}()
			}
			wg.Wait()
			return nil
		})
		if err != nil {
			status.Error(cliCtx, err.Error())
			return nil, cobra.ShellCompDirectiveError
		}
		return images.ListKeys(), cobra.ShellCompDirectiveNoSpace
	}
}
