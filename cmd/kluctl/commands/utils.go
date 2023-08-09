package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func withKluctlProjectFromArgs(ctx context.Context, projectFlags args.ProjectFlags, argsFlags *args.ArgsFlags, internalDeploy bool, strictTemplates bool, forCompletion bool, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
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

	var repoRoot string
	if !internalDeploy {
		repoRoot, err = git.DetectGitRepositoryRoot(projectDir)
		if err != nil {
			status.Warning(ctx, "Failed to detect git project root. This might cause follow-up errors")
		}
	}

	if repoRoot == "" {
		repoRoot = projectDir
	}

	ctx, cancel := context.WithTimeout(ctx, projectFlags.Timeout)
	defer cancel()

	sshPool := &ssh_pool.SshPool{}

	var repoOverrides []repocache.RepoOverride
	for _, x := range projectFlags.LocalGitOverride {
		ro, err := parseRepoOverride(ctx, x, false)
		if err != nil {
			return fmt.Errorf("invalid --local-git-override: %w", err)
		}
		repoOverrides = append(repoOverrides, ro)
	}
	for _, x := range projectFlags.LocalGitGroupOverride {
		ro, err := parseRepoOverride(ctx, x, true)
		if err != nil {
			return fmt.Errorf("invalid --local-git-group-override: %w", err)
		}
		repoOverrides = append(repoOverrides, ro)
	}

	messageCallbacks := &messages.MessageCallbacks{
		WarningFn:            func(s string) { status.Warning(ctx, s) },
		TraceFn:              func(s string) { status.Trace(ctx, s) },
		AskForPasswordFn:     func(s string) (string, error) { return prompts.AskForPassword(ctx, s) },
		AskForConfirmationFn: func(s string) bool { return prompts.AskForConfirmation(ctx, s) },
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
	commandResultFlags   *args.CommandResultFlags

	internalDeploy    bool
	forSeal           bool
	forCompletion     bool
	offlineKubernetes bool
	kubernetesVersion string
}

type commandCtx struct {
	ctx         context.Context
	targetCtx   *kluctl_project.TargetContext
	images      *deployment.Images
	resultStore results.ResultStore
}

func withProjectCommandContext(ctx context.Context, args projectTargetCommandArgs, cb func(cmdCtx *commandCtx) error) error {
	return withKluctlProjectFromArgs(ctx, args.projectFlags, &args.argsFlags, args.internalDeploy, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
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

	clientConfig, contextName, err := p.LoadK8sConfig(ctx, targetParams)
	if err != nil {
		return err
	}

	var k *k8s.K8sCluster
	if clientConfig != nil {
		discovery, mapper, err := k8s.CreateDiscoveryAndMapper(ctx, clientConfig)
		if err != nil {
			return err
		}

		s := status.Start(ctx, fmt.Sprintf("Initializing k8s client"))
		k, err = k8s.NewK8sCluster(ctx, clientConfig, discovery, mapper, targetParams.DryRun)
		if err != nil {
			s.Failed()
			return err
		}
		s.Success()
	}

	var resultStore results.ResultStore
	if args.commandResultFlags != nil && args.commandResultFlags.WriteCommandResult {
		client, err := client2.New(clientConfig, client2.Options{
			Mapper: mapper,
		})
		if err != nil {
			return err
		}

		resultStore, err = results.NewResultStoreSecrets(ctx, clientConfig, client, args.commandResultFlags.CommandResultNamespace, args.commandResultFlags.KeepCommandResultsCount, args.commandResultFlags.KeepValidateResultsCount)
		if err != nil {
			return err
		}
	}
	targetCtx, err := p.NewTargetContext(ctx, contextName, k, targetParams)
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
		ctx:         ctx,
		targetCtx:   targetCtx,
		images:      images,
		resultStore: resultStore,
	}

	return cb(cmdCtx)
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

func parseRepoOverride(ctx context.Context, s string, isGroup bool) (repocache.RepoOverride, error) {
	sp := strings.SplitN(s, "=", 2)
	if len(sp) != 2 {
		return repocache.RepoOverride{}, fmt.Errorf("%s", s)
	}

	repoKey, err := types.ParseGitRepoKey(sp[0])
	if err != nil {
		// try as legacy repo key
		u, err2 := types.ParseGitUrl(sp[0])
		if err2 != nil {
			// return original error
			return repocache.RepoOverride{}, err
		}

		x := u.Host
		if !strings.HasPrefix(u.Path, "/") {
			x += "/"
		}
		x += u.Path
		repoKey, err2 = types.ParseGitRepoKey(x)
		if err2 != nil {
			// return original error
			return repocache.RepoOverride{}, err
		}

		status.Deprecation(ctx, "old-repo-override", "Passing --local-git-override/--local-git-override-group in the example.com:path form is deprecated and will not be supported in future versions of Kluctl. Please use the example.com/path form.")
	}

	return repocache.RepoOverride{
		RepoKey:  repoKey,
		IsGroup:  isGroup,
		Override: sp[1],
	}, nil
}
