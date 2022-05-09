package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/registries"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func withKluctlProjectFromArgs(projectFlags args.ProjectFlags, strictTemplates bool, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
	var url *git_url.GitUrl
	if projectFlags.ProjectUrl != "" {
		var err error
		url, err = git_url.Parse(projectFlags.ProjectUrl)
		if err != nil {
			return err
		}
	}

	tmpDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "project-")
	if err != nil {
		return fmt.Errorf("creating temporary project directory failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	j2, err := jinja2.NewJinja2()
	if err != nil {
		return err
	}
	defer j2.Close()

	j2.SetStrict(strictTemplates)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := git.DetectGitRepositoryRoot(cwd)
	if err != nil && projectFlags.FromArchive == "" {
		status.Warning(cliCtx, "Failed to detect git project root. This might cause follow-up errors")
	}

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		RepoRoot:            repoRoot,
		ProjectDir:          cwd,
		ProjectUrl:          url,
		ProjectRef:          projectFlags.ProjectRef,
		ProjectConfig:       projectFlags.ProjectConfig.String(),
		LocalClusters:       projectFlags.LocalClusters.String(),
		LocalDeployment:     projectFlags.LocalDeployment.String(),
		LocalSealedSecrets:  projectFlags.LocalSealedSecrets.String(),
		FromArchive:         projectFlags.FromArchive.String(),
		FromArchiveMetadata: projectFlags.FromArchiveMetadata.String(),
		AllowGitClone:       projectFlags.FromArchive == "",
		GitAuthProviders:    auth.NewDefaultAuthProviders(),
		GitUpdateInterval:   projectFlags.GitCacheUpdateInterval,
	}

	ctx, cancel := context.WithTimeout(cliCtx, projectFlags.Timeout)
	defer cancel()
	p, err := kluctl_project.LoadKluctlProject(ctx, loadArgs, tmpDir, j2)
	if err != nil {
		return err
	}
	if projectFlags.OutputMetadata != "" {
		md := p.GetMetadata()
		b, err := yaml.WriteYamlBytes(md)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(projectFlags.OutputMetadata.String(), b, 0o640)
		if err != nil {
			return err
		}
	}
	return cb(ctx, p)
}

type projectTargetCommandArgs struct {
	projectFlags         args.ProjectFlags
	targetFlags          args.TargetFlags
	argsFlags            args.ArgsFlags
	imageFlags           args.ImageFlags
	inclusionFlags       args.InclusionFlags
	dryRunArgs           *args.DryRunFlags
	renderOutputDirFlags args.RenderOutputDirFlags

	forSeal       bool
	forCompletion bool
}

type commandCtx struct {
	ctx       context.Context
	targetCtx *kluctl_project.TargetContext
	images    *deployment.Images
}

func withProjectCommandContext(args projectTargetCommandArgs, cb func(ctx *commandCtx) error) error {
	return withKluctlProjectFromArgs(args.projectFlags, true, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		return withProjectTargetCommandContext(ctx, args, p, cb)
	})
}

func withProjectTargetCommandContext(ctx context.Context, args projectTargetCommandArgs, p *kluctl_project.LoadedKluctlProject, cb func(ctx *commandCtx) error) error {
	rh := registries.NewRegistryHelper(ctx)
	err := rh.ParseAuthEntriesFromEnv()
	if err != nil {
		return fmt.Errorf("failed to parse registry auth from environment: %w", err)
	}
	images, err := deployment.NewImages(rh, args.imageFlags.UpdateImages, args.forCompletion)
	if err != nil {
		return err
	}
	fixedImages, err := args.imageFlags.LoadFixedImagesFromArgs()
	if err != nil {
		return err
	}
	for _, fi := range fixedImages {
		images.AddFixedImage(fi)
	}

	inclusion, err := args.inclusionFlags.ParseInclusionFromArgs()
	if err != nil {
		return err
	}

	optionArgs, err := deployment.ParseArgs(args.argsFlags.Arg)
	if err != nil {
		return err
	}

	renderOutputDir := args.renderOutputDirFlags.RenderOutputDir.String()
	if renderOutputDir == "" {
		tmpDir, err := ioutil.TempDir(p.TmpDir, "rendered")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		renderOutputDir = tmpDir
	}

	clientConfigGetter := func(context string) (*rest.Config, error) {
		if args.forCompletion {
			return nil, nil
		}
		configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{CurrentContext: context}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides).ClientConfig()
	}

	targetCtx, err := p.NewTargetContext(ctx, clientConfigGetter, args.targetFlags.Target, args.projectFlags.Cluster,
		args.dryRunArgs == nil || args.dryRunArgs.DryRun || args.forCompletion,
		optionArgs, args.forSeal, images, inclusion,
		renderOutputDir)
	if err != nil {
		return err
	}

	if !args.forSeal && !args.forCompletion {
		err = targetCtx.DeploymentCollection.Prepare(targetCtx.K)
		if err != nil {
			return err
		}
	}

	cmdCtx := &commandCtx{
		ctx:       ctx,
		targetCtx: targetCtx,
		images:    images,
	}

	return cb(cmdCtx)
}
