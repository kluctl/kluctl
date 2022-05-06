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
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
)

func withKluctlProjectFromArgs(projectFlags args.ProjectFlags, strictTemplates bool, cb func(p *kluctl_project.LoadedKluctlProject) error) error {
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
		log.Warning("Failed to detect git project root. This might cause follow-up errors")
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

	loadCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(projectFlags.LoadTimeout))
	defer cancel()
	p, err := kluctl_project.LoadKluctlProject(loadCtx, loadArgs, tmpDir, j2)
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
	return cb(p)
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
	targetCtx *kluctl_project.TargetContext
	images    *deployment.Images
}

func withProjectCommandContext(args projectTargetCommandArgs, cb func(ctx *commandCtx) error) error {
	return withKluctlProjectFromArgs(args.projectFlags, true, func(p *kluctl_project.LoadedKluctlProject) error {
		return withProjectTargetCommandContext(args, p, cb)
	})
}

func withProjectTargetCommandContext(args projectTargetCommandArgs, p *kluctl_project.LoadedKluctlProject, cb func(ctx *commandCtx) error) error {
	rh := registries.NewRegistryHelper()
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

	ctx, err := p.NewTargetContext(clientConfigGetter, args.targetFlags.Target, args.projectFlags.Cluster,
		args.dryRunArgs == nil || args.dryRunArgs.DryRun || args.forCompletion,
		optionArgs, args.forSeal, images, inclusion,
		renderOutputDir)
	if err != nil {
		return err
	}

	if !args.forSeal && !args.forCompletion {
		err = ctx.DeploymentCollection.Prepare(ctx.K)
		if err != nil {
			return err
		}
	}

	cmdCtx := &commandCtx{
		targetCtx: ctx,
		images:    images,
	}

	return cb(cmdCtx)
}
