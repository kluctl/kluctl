package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"reflect"
	"time"
)

func RegisterFlagCompletionFuncs(cmd interface{}, ccmd *cobra.Command) error {
	v := reflect.ValueOf(cmd).Elem()
	projectFlags := v.FieldByName("ProjectFlags")
	targetFlags := v.FieldByName("TargetFlags")

	if projectFlags.IsValid() && targetFlags.IsValid() {
		_ = ccmd.RegisterFlagCompletionFunc("target", buildTargetCompletionFunc(projectFlags.Addr().Interface().(*args.ProjectFlags)))
	}

	return nil
}

func buildTargetCompletionFunc(projectArgs *args.ProjectFlags) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var ret []string
		// let's not update git caches too often
		projectArgs.GitCacheUpdateInterval = time.Second * 60
		err := withKluctlProjectFromArgs(*projectArgs, func(p *kluctl_project.KluctlProjectContext) error {
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
