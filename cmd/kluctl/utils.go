package main

import (
	"fmt"
	args "github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/codablock/kluctl/pkg/jinja2_server"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/kluctl_project"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"io/ioutil"
	"os"
	"path/filepath"
)

func withKluctlProjectFromArgs(cb func(p *kluctl_project.KluctlProjectContext) error) error {
	var url *git_url.GitUrl
	if args.ProjectUrl != "" {
		var err error
		url, err = git_url.Parse(args.ProjectUrl)
		if err != nil {
			return err
		}
	}
	js, err := jinja2_server.NewJinja2Server()
	if err != nil {
		return err
	}
	defer js.Stop()
	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		ProjectUrl:          url,
		ProjectRef:          args.ProjectRef,
		ProjectConfig:       args.ProjectConfig,
		LocalClusters:       args.LocalClusters,
		LocalDeployment:     args.LocalDeployment,
		LocalSealedSecrets:  args.LocalSealedSecrets,
		FromArchive:         args.FromArchive,
		FromArchiveMetadata: args.FromArchiveMetadata,
		JS:                  js,
	}
	return kluctl_project.LoadKluctlProject(loadArgs, cb)
}

type commandCtx struct {
	kluctlProject        *kluctl_project.KluctlProjectContext
	target               *types.Target
	clusterConfig        *types.ClusterConfig
	k                    *k8s.K8sCluster
	deploymentProject    *deployment.DeploymentProject
	deploymentCollection *deployment.DeploymentCollection
}

func withProjectCommandContext(cb func(ctx *commandCtx) error) error {
	return withKluctlProjectFromArgs(func(p *kluctl_project.KluctlProjectContext) error {
		var target *types.Target
		if args.Target != "" {
			t, err := p.FindTarget(args.Target)
			if err != nil {
				return err
			}
			target = t
		}
		return withProjectTargetCommandContext(p, target, false, cb)
	})
}

func withProjectTargetCommandContext(p *kluctl_project.KluctlProjectContext, target *types.Target, forSeal bool, cb func(ctx *commandCtx) error) error {
	deploymentDir, err := filepath.Abs(p.DeploymentDir)
	if err != nil {
		return err
	}

	clusterName := args.Cluster
	if clusterName == "" {
		if target == nil {
			return fmt.Errorf("you must specify an existing --cluster when not providing a --target")
		}
		clusterName = target.Cluster
	}

	clusterConfig, err := p.LoadClusterConfig(clusterName)
	if err != nil {
		return err
	}

	k, err := k8s.NewK8sCluster(clusterConfig.Cluster.Context, args.DryRun)
	if err != nil {
		return err
	}

	varsCtx := deployment.NewVarsCtx(p.JS)
	err = varsCtx.UpdateChildFromStruct("cluster", clusterConfig.Cluster)
	if err != nil {
		return err
	}

	images, err := deployment.NewImages(args.UpdateImages)
	if err != nil {
		return err
	}

	inclusion, err := args.ParseInclusionFromArgs()
	if err != nil {
		return err
	}

	allArgs := make(map[string]interface{})

	optionArgs, err := deployment.ParseArgs(args.Args)
	if err != nil {
		return err
	}
	if target != nil {
		for argName, argValue := range optionArgs {
			err = p.CheckDynamicArg(target, argName, argValue)
			if err != nil {
				return err
			}
		}
	}
	utils.MergeObject(allArgs, deployment.ConvertArgsToVars(optionArgs))
	if target != nil {
		utils.MergeObject(allArgs, target.Args)
		if forSeal {
			utils.MergeObject(allArgs, target.SealingConfig.Args)
		}
	}

	err = deployment.CheckRequiredDeployArgs(deploymentDir, varsCtx, allArgs)
	if err != nil {
		return err
	}

	varsCtx.UpdateChild("args", allArgs)

	renderOutputDir := args.RenderOutputDir
	if renderOutputDir == "" {
		tmpDir, err := ioutil.TempDir(p.TmpDir, "rendered")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		renderOutputDir = tmpDir
	}

	d, err := deployment.NewDeploymentProject(k, varsCtx, deploymentDir, p.SealedSecretsDir, nil)
	if err != nil {
		return err
	}
	c, err := deployment.NewDeploymentCollection(d, images, inclusion, renderOutputDir, forSeal)
	if err != nil {
		return err
	}

	fixedImages, err := args.LoadFixedImagesFromArgs()
	if err != nil {
		return err
	}
	if target != nil {
		for _, fi := range target.Images {
			images.AddFixedImage(fi)
		}
	}
	for _, fi := range fixedImages.Images {
		images.AddFixedImage(fi)
	}

	if !forSeal {
		err = c.Prepare(k)
		if err != nil {
			return err
		}
	}

	ctx := &commandCtx{
		kluctlProject:        p,
		target:               target,
		clusterConfig:        clusterConfig,
		k:                    k,
		deploymentProject:    d,
		deploymentCollection: c,
	}

	return cb(ctx)
}
