package commands

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func withKluctlProjectFromArgs(ctx context.Context, projectFlags args.ProjectFlags, argsFlags *args.ArgsFlags, helmCredentials *args.HelmCredentials, registryCredentials *args.RegistryCredentials, internalDeploy bool, strictTemplates bool, forCompletion bool, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
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

	var gitRepoOverrides []repocache.RepoOverride
	var ociRepoOverrides []repocache.RepoOverride
	for _, x := range projectFlags.LocalGitOverride {
		ro, err := parseRepoOverride(ctx, x, false, "git", true)
		if err != nil {
			return fmt.Errorf("invalid --local-git-override: %w", err)
		}
		gitRepoOverrides = append(gitRepoOverrides, ro)
	}
	for _, x := range projectFlags.LocalGitGroupOverride {
		ro, err := parseRepoOverride(ctx, x, true, "git", true)
		if err != nil {
			return fmt.Errorf("invalid --local-git-group-override: %w", err)
		}
		gitRepoOverrides = append(gitRepoOverrides, ro)
	}
	for _, x := range projectFlags.LocalOciOverride {
		ro, err := parseRepoOverride(ctx, x, false, "oci", false)
		if err != nil {
			return fmt.Errorf("invalid --local-oci-override: %w", err)
		}
		ociRepoOverrides = append(ociRepoOverrides, ro)
	}
	for _, x := range projectFlags.LocalOciGroupOverride {
		ro, err := parseRepoOverride(ctx, x, true, "oci", false)
		if err != nil {
			return fmt.Errorf("invalid --local-oci-group-override: %w", err)
		}
		ociRepoOverrides = append(ociRepoOverrides, ro)
	}

	messageCallbacks := &messages.MessageCallbacks{
		WarningFn:            func(s string) { status.Warning(ctx, s) },
		TraceFn:              func(s string) { status.Trace(ctx, s) },
		AskForPasswordFn:     func(s string) (string, error) { return prompts.AskForPassword(ctx, s) },
		AskForConfirmationFn: func(s string) bool { return prompts.AskForConfirmation(ctx, s) },
	}
	gitAuth := auth.NewDefaultAuthProviders("KLUCTL_GIT", messageCallbacks)
	ociAuth := auth_provider.NewDefaultAuthProviders("KLUCTL_REGISTRY")
	helmAuth := helm_auth.NewDefaultAuthProviders("KLUCTL_HELM")
	if x, err := helmCredentials.BuildAuthProvider(ctx); err != nil {
		return err
	} else {
		helmAuth.RegisterAuthProvider(x, false)
	}
	if x, err := registryCredentials.BuildAuthProvider(ctx); err != nil {
		return err
	} else {
		ociAuth.RegisterAuthProvider(x, false)
	}

	gitRp := repocache.NewGitRepoCache(ctx, sshPool, gitAuth, gitRepoOverrides, projectFlags.GitCacheUpdateInterval)
	defer gitRp.Clear()

	ociRp := repocache.NewOciRepoCache(ctx, ociAuth, ociRepoOverrides, projectFlags.GitCacheUpdateInterval)
	defer gitRp.Clear()

	externalArgs, err := argsFlags.LoadArgs()
	if err != nil {
		return err
	}

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		RepoRoot:           repoRoot,
		ProjectDir:         projectDir,
		ProjectConfig:      projectFlags.ProjectConfig.String(),
		ExternalArgs:       externalArgs,
		GitRP:              gitRp,
		OciRP:              ociRp,
		OciAuthProvider:    ociAuth,
		HelmAuthProvider:   helmAuth,
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
	registryCredentials  args.RegistryCredentials
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
	ctx       context.Context
	targetCtx *kluctl_project.TargetContext
	images    *deployment.Images

	resultId    string
	resultStore results.ResultStore
}

func withProjectCommandContext(ctx context.Context, args projectTargetCommandArgs, cb func(cmdCtx *commandCtx) error) error {
	return withKluctlProjectFromArgs(ctx, args.projectFlags, &args.argsFlags, &args.helmCredentials, &args.registryCredentials, args.internalDeploy, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
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
		OciAuthProvider:    p.LoadArgs.OciAuthProvider,
		HelmAuthProvider:   p.LoadArgs.HelmAuthProvider,
		RenderOutputDir:    renderOutputDir,
	}

	commandResultId := uuid.NewString()

	clientConfig, contextName, err := p.LoadK8sConfig(ctx, targetParams)
	if err != nil {
		return err
	}

	var k *k8s.K8sCluster
	var resultStore results.ResultStore
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

		resultStore, err = initStores(ctx, clientConfig, mapper, args.commandResultFlags)
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
		resultId:    commandResultId,
		resultStore: resultStore,
	}

	return cb(cmdCtx)
}

func initStores(ctx context.Context, clientConfig *rest.Config, mapper meta.RESTMapper, flags *args.CommandResultFlags) (results.ResultStore, error) {
	if flags == nil || !flags.WriteCommandResult {
		return nil, nil
	}

	client, err := client2.New(clientConfig, client2.Options{
		Mapper: mapper,
	})
	if err != nil {
		return nil, err
	}

	resultStore, err := results.NewResultStoreSecrets(ctx, clientConfig, client, flags.CommandResultNamespace, flags.KeepCommandResultsCount, flags.KeepValidateResultsCount)
	if err != nil {
		return nil, err
	}

	return resultStore, nil
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

func parseRepoOverride(ctx context.Context, s string, isGroup bool, type_ string, allowLegacy bool) (repocache.RepoOverride, error) {
	sp := strings.SplitN(s, "=", 2)
	if len(sp) != 2 {
		return repocache.RepoOverride{}, fmt.Errorf("%s", s)
	}

	repoKey, err := types.ParseRepoKey(sp[0], type_)
	if err != nil {
		if !allowLegacy {
			return repocache.RepoOverride{}, err
		}

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
		repoKey, err2 = types.ParseRepoKey(x, type_)
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

func buildResultStore(ctx context.Context, restConfig *rest.Config, args args.CommandResultFlags, startCleanup bool) (results.ResultStore, error) {
	if !args.WriteCommandResult {
		return nil, nil
	}

	c, err := client2.NewWithWatch(restConfig, client2.Options{})
	if err != nil {
		return nil, err
	}

	resultStore, err := results.NewResultStoreSecrets(ctx, restConfig, c, args.CommandResultNamespace, args.KeepCommandResultsCount, args.KeepValidateResultsCount)
	if err != nil {
		return nil, err
	}

	if startCleanup {
		err = resultStore.StartCleanupOrphans()
		if err != nil {
			return nil, err
		}
	}

	return resultStore, nil
}
