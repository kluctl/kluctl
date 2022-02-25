package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/codablock/kluctl/pkg/jinja2"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/kluctl_project"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"io/ioutil"
	"os"
	"path/filepath"
)

func withKluctlProjectFromArgs(projectFlags args.ProjectFlags, cb func(p *kluctl_project.KluctlProjectContext) error) error {
	var url *git_url.GitUrl
	if projectFlags.ProjectUrl != "" {
		var err error
		url, err = git_url.Parse(projectFlags.ProjectUrl)
		if err != nil {
			return err
		}
	}
	j2, err := jinja2.NewJinja2()
	if err != nil {
		return err
	}
	defer j2.Close()

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		ProjectUrl:          url,
		ProjectRef:          projectFlags.ProjectRef,
		ProjectConfig:       projectFlags.ProjectConfig,
		LocalClusters:       projectFlags.LocalClusters,
		LocalDeployment:     projectFlags.LocalDeployment,
		LocalSealedSecrets:  projectFlags.LocalSealedSecrets,
		FromArchive:         projectFlags.FromArchive,
		FromArchiveMetadata: projectFlags.FromArchiveMetadata,
		J2:                  j2,
	}
	return kluctl_project.LoadKluctlProject(loadArgs, cb)
}

type projectTargetCommandArgs struct {
	projectFlags         args.ProjectFlags
	targetFlags          args.TargetFlags
	argsFlags            args.ArgsFlags
	imageFlags           args.ImageFlags
	inclusionFlags       args.InclusionFlags
	dryRunArgs           *args.DryRunFlags
	renderOutputDirFlags args.RenderOutputDirFlags
}

type commandCtx struct {
	kluctlProject        *kluctl_project.KluctlProjectContext
	target               *types.Target
	clusterConfig        *types.ClusterConfig
	k                    *k8s.K8sCluster
	images               *deployment.Images
	deploymentProject    *deployment.DeploymentProject
	deploymentCollection *deployment.DeploymentCollection
}

func withProjectCommandContext(args projectTargetCommandArgs, cb func(ctx *commandCtx) error) error {
	return withKluctlProjectFromArgs(args.projectFlags, func(p *kluctl_project.KluctlProjectContext) error {
		var target *types.Target
		if args.targetFlags.Target != "" {
			t, err := p.FindDynamicTarget(args.targetFlags.Target)
			if err != nil {
				return err
			}
			target = t.Target
		}
		return withProjectTargetCommandContext(args, p, target, false, cb)
	})
}

func withProjectTargetCommandContext(args projectTargetCommandArgs, p *kluctl_project.KluctlProjectContext, target *types.Target, forSeal bool, cb func(ctx *commandCtx) error) error {
	deploymentDir, err := filepath.Abs(p.DeploymentDir)
	if err != nil {
		return err
	}

	clusterName := args.projectFlags.Cluster
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

	k, err := k8s.NewK8sCluster(clusterConfig.Cluster.Context, args.dryRunArgs == nil || args.dryRunArgs.DryRun)
	if err != nil {
		return err
	}

	varsCtx := jinja2.NewVarsCtx(p.J2)
	err = varsCtx.UpdateChildFromStruct("cluster", clusterConfig.Cluster)
	if err != nil {
		return err
	}

	images, err := deployment.NewImages(args.imageFlags.UpdateImages)
	if err != nil {
		return err
	}

	inclusion, err := args.inclusionFlags.ParseInclusionFromArgs()
	if err != nil {
		return err
	}

	allArgs := uo.New()

	optionArgs, err := deployment.ParseArgs(args.argsFlags.Arg)
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
	allArgs.Merge(deployment.ConvertArgsToVars(optionArgs))
	if target != nil {
		if target.Args != nil {
			allArgs.Merge(target.Args)
		}
		if forSeal {
			if target.SealingConfig.Args != nil {
				allArgs.Merge(target.SealingConfig.Args)
			}
		}
	}

	err = deployment.CheckRequiredDeployArgs(deploymentDir, varsCtx, allArgs)
	if err != nil {
		return err
	}

	varsCtx.UpdateChild("args", allArgs)

	renderOutputDir := args.renderOutputDirFlags.RenderOutputDir
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

	fixedImages, err := args.imageFlags.LoadFixedImagesFromArgs()
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
		images:               images,
		deploymentProject:    d,
		deploymentCollection: c,
	}

	return cb(ctx)
}
