package main

import (
	"embed"
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
)

var (
	// files with _ as prefix are excluded by default, this is a workaround until go 1.18 is released
	// see https://github.com/golang/go/issues/43854
	//go:embed bootstrap bootstrap/sealed-secrets/charts/sealed-secrets/templates/*
	embedBootstrap embed.FS
)

func runCmdBootstrap(cmd *cobra.Command, args_ []string) error {
	tmpBootstrapDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "bootstrap-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpBootstrapDir)

	err = utils.FsCopyDir(embedBootstrap, "bootstrap", tmpBootstrapDir)
	if err != nil {
		return err
	}

	if args.LocalDeployment == "" {
		args.LocalDeployment = tmpBootstrapDir
	}

	return withProjectCommandContext(func(ctx *commandCtx) error {
		existing, _, err := ctx.k.GetSingleObject(types.ObjectRef{
			GVK:  schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
			Name: "sealedsecrets.bitnami.com",
		})
		if existing != nil {
			component, _ := existing.GetLabels()["kluctl.io/component"]
			if !args.ForceYes && component != "bootstrap" {
				if !AskForConfirmation("It looks like you're trying to bootstrap a cluster that already has the sealed-secrets deployed but not managed by kluctl. Do you really want to continue bootstrapping?") {
					return fmt.Errorf("aborted")
				}
			}
		}

		err = runCmdDeploy2(cmd, ctx)
		if err != nil {
			return err
		}
		err = runCmdPrune2(cmd, ctx)
		if err != nil {
			return err
		}
		return nil
	})
}

func init() {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a target cluster",
		Long: "This will install the sealed-secrets operator into the specified cluster if not already " +
			"installed.\n\n" +
			"Either --target or --cluster must be specified.",
		RunE: runCmdBootstrap,
	}

	args.AddProjectArgs(cmd, false, true, true)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Yes:            true,
		DryRun:         true,
		ForceApply:     true,
		ReplaceOnError: true,
		HookTimeout:    true,
		AbortOnError:   true,
		OutputFormat:   true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
