package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"os"
	"strings"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func withKluctlProjectFromArgs(ctx context.Context, projectFlags args.ProjectFlags, argsFlags *args.ArgsFlags, strictTemplates bool, forCompletion bool, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
	tmpDir, err := os.MkdirTemp(utils.GetTmpBaseDir(ctx), "project-")
	if err != nil {
		return fmt.Errorf("creating temporary project directory failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	j2, err := kluctl_jinja2.NewKluctlJinja2(strictTemplates)
	if err != nil {
		return err
	}
	defer j2.Close()

	projectDir, err := projectFlags.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}

	repoRoot, err := git.DetectGitRepositoryRoot(projectDir)
	if err != nil {
		status.Warning(ctx, "Failed to detect git project root. This might cause follow-up errors")
	}

	ctx, cancel := context.WithTimeout(ctx, projectFlags.Timeout)
	defer cancel()

	sshPool := &ssh_pool.SshPool{}

	var repoOverrides []repocache.RepoOverride
	for _, x := range projectFlags.LocalGitOverride {
		ro, err := parseRepoOverride(x, false)
		if err != nil {
			return fmt.Errorf("invalid --local-git-override: %w", err)
		}
		repoOverrides = append(repoOverrides, ro)
	}
	for _, x := range projectFlags.LocalGitGroupOverride {
		ro, err := parseRepoOverride(x, true)
		if err != nil {
			return fmt.Errorf("invalid --local-git-group-override: %w", err)
		}
		repoOverrides = append(repoOverrides, ro)
	}

	messageCallbacks := &messages.MessageCallbacks{
		WarningFn:            func(s string) { status.Warning(ctx, s) },
		TraceFn:              func(s string) { status.Trace(ctx, s) },
		AskForPasswordFn:     func(s string) (string, error) { return status.AskForPassword(ctx, s) },
		AskForConfirmationFn: func(s string) bool { return status.AskForConfirmation(ctx, s) },
	}
	gitAuth := auth.NewDefaultAuthProviders("KLUCTL_GIT", messageCallbacks)

	rp := repocache.NewGitRepoCache(ctx, sshPool, gitAuth, repoOverrides, projectFlags.GitCacheUpdateInterval)
	defer rp.Clear()

	var externalArgs *uo.UnstructuredObject
	if argsFlags != nil {
		optionArgs, err := deployment.ParseArgs(argsFlags.Arg)
		if err != nil {
			return err
		}
		externalArgs, err = deployment.ConvertArgsToVars(optionArgs, true)
		if err != nil {
			return err
		}
		for _, a := range argsFlags.ArgsFromFile {
			optionArgs2, err := uo.FromFile(a)
			if err != nil {
				return err
			}
			externalArgs.Merge(optionArgs2)
		}
	}

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		RepoRoot:           repoRoot,
		ProjectDir:         projectDir,
		ProjectConfig:      projectFlags.ProjectConfig.String(),
		ExternalArgs:       externalArgs,
		RP:                 rp,
		ClientConfigGetter: clientConfigGetter(forCompletion),
	}

	p, err := kluctl_project.LoadKluctlProject(ctx, loadArgs, tmpDir, j2)
	if err != nil {
		return err
	}

	return cb(ctx, p)
}

type projectTargetCommandArgs struct {
	projectFlags         args.ProjectFlags
	targetFlags          args.TargetFlags
	argsFlags            args.ArgsFlags
	imageFlags           args.ImageFlags
	inclusionFlags       args.InclusionFlags
	helmCredentials      args.HelmCredentials
	dryRunArgs           *args.DryRunFlags
	renderOutputDirFlags args.RenderOutputDirFlags

	forSeal           bool
	forCompletion     bool
	offlineKubernetes bool
	kubernetesVersion string
}

type commandCtx struct {
	ctx       context.Context
	targetCtx *kluctl_project.TargetContext
	images    *deployment.Images
}

func withProjectCommandContext(ctx context.Context, args projectTargetCommandArgs, cb func(cmdCtx *commandCtx) error) error {
	return withKluctlProjectFromArgs(ctx, args.projectFlags, &args.argsFlags, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		return withProjectTargetCommandContext(ctx, args, p, cb)
	})
}

func withProjectTargetCommandContext(ctx context.Context, args projectTargetCommandArgs, p *kluctl_project.LoadedKluctlProject, cb func(cmdCtx *commandCtx) error) error {
	images, err := deployment.NewImages()
	if err != nil {
		return err
	}
	fixedImages, err := args.imageFlags.LoadFixedImagesFromArgs()
	if err != nil {
		return err
	}
	images.PrependFixedImages(fixedImages)

	inclusion, err := args.inclusionFlags.ParseInclusionFromArgs()
	if err != nil {
		return err
	}

	renderOutputDir := args.renderOutputDirFlags.RenderOutputDir
	if renderOutputDir == "" {
		tmpDir, err := os.MkdirTemp(p.TmpDir, "rendered")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		renderOutputDir = tmpDir
	}

	targetParams := kluctl_project.TargetContextParams{
		TargetName:         args.targetFlags.Target,
		TargetNameOverride: args.targetFlags.TargetNameOverride,
		ContextOverride:    args.targetFlags.Context,
		OfflineK8s:         args.offlineKubernetes,
		K8sVersion:         args.kubernetesVersion,
		DryRun:             args.dryRunArgs == nil || args.dryRunArgs.DryRun || args.forCompletion,
		ForSeal:            args.forSeal,
		Images:             images,
		Inclusion:          inclusion,
		HelmCredentials:    &args.helmCredentials,
		RenderOutputDir:    renderOutputDir,
	}

	targetCtx, err := p.NewTargetContext(ctx, targetParams)
	if err != nil {
		return err
	}

	if !args.forSeal && !args.forCompletion {
		err = targetCtx.DeploymentCollection.Prepare()
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

func addCommandInfo(r *result.CommandResult, command string, ctx *commandCtx, targetFlags *args.TargetFlags,
	imageFlags *args.ImageFlags, inclusionFlags *args.InclusionFlags,
	dryRunFlags *args.DryRunFlags, forceApplyFlags *args.ForceApplyFlags, replaceOnErrorFlags *args.ReplaceOnErrorFlags, abortOnErrorFlags *args.AbortOnErrorFlags, noWait bool) error {
	r.Command = &result.CommandInfo{
		Initiator: result.CommandInititiator_CommandLine,
		Command:   command,
		Target:    ctx.targetCtx.Target,
		Args:      ctx.targetCtx.KluctlProject.LoadArgs.ExternalArgs,
		NoWait:    noWait,
	}
	if targetFlags != nil {
		r.Command.TargetNameOverride = targetFlags.TargetNameOverride
		r.Command.ContextOverride = targetFlags.Context
	}
	if imageFlags != nil {
		var err error
		r.Command.Images, err = imageFlags.LoadFixedImagesFromArgs()
		if err != nil {
			return err
		}
	}
	if inclusionFlags != nil {
		r.Command.IncludeTags = inclusionFlags.IncludeTag
		r.Command.ExcludeTags = inclusionFlags.ExcludeTag
		r.Command.IncludeDeploymentDirs = inclusionFlags.IncludeDeploymentDir
		r.Command.ExcludeDeploymentDirs = inclusionFlags.ExcludeDeploymentDir
	}
	if dryRunFlags != nil {
		r.Command.DryRun = dryRunFlags.DryRun
	}
	if forceApplyFlags != nil {
		r.Command.ForceApply = forceApplyFlags.ForceApply
	}
	if replaceOnErrorFlags != nil {
		r.Command.ReplaceOnError = replaceOnErrorFlags.ReplaceOnError
		r.Command.ForceReplaceOnError = replaceOnErrorFlags.ForceReplaceOnError
	}
	if abortOnErrorFlags != nil {
		r.Command.AbortOnError = abortOnErrorFlags.AbortOnError
	}
	r.Deployment = &ctx.targetCtx.DeploymentProject.Config
	r.RenderedObjects = ctx.targetCtx.DeploymentCollection.LocalObjects()
	return nil
}

func clientConfigGetter(forCompletion bool) func(context *string) (*rest.Config, *api.Config, error) {
	return func(context *string) (*rest.Config, *api.Config, error) {
		if forCompletion {
			return nil, nil, nil
		}

		configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		if context != nil {
			configOverrides.CurrentContext = *context
		}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)
		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			return nil, nil, err
		}
		if context != nil {
			rawConfig.CurrentContext = *context
		}
		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, err
		}
		return restConfig, &rawConfig, nil
	}
}

func parseRepoOverride(s string, isGroup bool) (ret repocache.RepoOverride, err error) {
	ret.IsGroup = isGroup

	sp := strings.SplitN(s, "=", 2)
	if len(sp) != 2 {
		return repocache.RepoOverride{}, fmt.Errorf("%s", s)
	}

	u, err := git_url.Parse(sp[0])
	if err != nil {
		// we need to prepend a dummy scheme to the repo key so that it is properly parsed
		dummyUrl := fmt.Sprintf("git://%s", sp[0])
		u, err = git_url.Parse(dummyUrl)
		if err != nil {
			return repocache.RepoOverride{}, fmt.Errorf("%s: %w", s, err)
		}
	}

	u = u.Normalize()
	ret.RepoUrl = *u
	ret.Override = sp[1]
	return
}
