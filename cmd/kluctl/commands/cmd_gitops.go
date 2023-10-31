package commands

import (
	"bytes"
	"context"
	"fmt"
	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	flag "github.com/spf13/pflag"
	v12 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"math/rand"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"sync"
	"time"
)

type gitopsCmd struct {
	Suspend   GitopsSuspendCmd   `cmd:"" help:"Suspend a GitOps deployment"`
	Resume    gitopsResumeCmd    `cmd:"" help:"Resume a GitOps deployment"`
	Reconcile gitopsReconcileCmd `cmd:"" help:"Trigger a GitOps reconciliation"`
	Diff      gitopsDiffCmd      `cmd:"" help:"Trigger a GitOps diff"`
	Deploy    gitopsDeployCmd    `cmd:"" help:"Trigger a GitOps deployment"`
	Prune     gitopsPruneCmd     `cmd:"" help:"Trigger a GitOps prune"`
	Validate  gitopsValidateCmd  `cmd:"" help:"Trigger a GitOps validate"`
	Logs      gitopsLogsCmd      `cmd:"" help:"Show logs from controller"`
}

type gitopsCmdHelper struct {
	args            args.GitOpsArgs
	logsArgs        args.GitOpsLogArgs
	overridableArgs args.GitOpsOverridableArgs

	noArgsReact noArgsReact

	projectGitRoot string
	projectGitInfo *result.GitInfo
	projectKey     *result.ProjectKey

	kds []v1beta1.KluctlDeployment

	restConfig   *rest.Config
	restMapper   meta.RESTMapper
	client       client.Client
	corev1Client *v1.CoreV1Client

	soResolver *sourceoverride.Manager
	soClient   *sourceoverride.ProxyClientCli

	resultStore results.ResultStore

	logsMutex          sync.Mutex
	logsBufs           map[logsKey]*logsBuf
	lastFlushedLogsKey *logsKey
}

type logsKey struct {
	objectKey   client.ObjectKey
	reconcileId string
}

type logsBuf struct {
	lastFlushTime time.Time
	lines         []logs.LogLine
}

type noArgsReact int

const (
	noArgsForbid noArgsReact = iota
	noArgsNoDeployments
	noArgsAllDeployments
	noArgsAutoDetectProject
	noArgsAutoDetectProjectAsk
)

func (g *gitopsCmdHelper) init(ctx context.Context) error {
	err := g.collectLocalProjectInfo(ctx)
	if err != nil {
		status.Warningf(ctx, "Failed to collect local project info: %s", err.Error())
	}

	g.logsBufs = map[logsKey]*logsBuf{}

	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: g.args.Context,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		configOverrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	defaultNs, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}

	_, mapper, err := k8s.CreateDiscoveryAndMapper(ctx, restConfig)
	if err != nil {
		return err
	}

	g.restConfig = restConfig
	g.restMapper = mapper

	g.client, err = client.NewWithWatch(restConfig, client.Options{
		Mapper: mapper,
	})
	if err != nil {
		return err
	}
	g.corev1Client, err = v1.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	err = g.checkCRDSupport(ctx)
	if err != nil {
		return err
	}

	ns := g.args.Namespace
	if ns == "" {
		ns = defaultNs
	}

	if g.args.Name != "" {
		if ns == "" {
			return fmt.Errorf("no namespace specified")
		}
		if g.args.LabelSelector != "" {
			return fmt.Errorf("either name or label selector must be set, not both")
		}

		var kd v1beta1.KluctlDeployment
		err = g.client.Get(ctx, client.ObjectKey{Name: g.args.Name, Namespace: ns}, &kd)
		if err != nil {
			return err
		}
		g.kds = append(g.kds, kd)
	} else if g.args.LabelSelector != "" {
		label, err := metav1.ParseToLabelSelector(g.args.LabelSelector)
		if err != nil {
			return err
		}
		sel, err := metav1.LabelSelectorAsSelector(label)
		if err != nil {
			return err
		}

		var opts []client.ListOption
		opts = append(opts, client.MatchingLabelsSelector{Selector: sel})
		if ns != "" {
			opts = append(opts, client.InNamespace(ns))
		}

		var l v1beta1.KluctlDeploymentList
		err = g.client.List(ctx, &l, opts...)
		if err != nil {
			return err
		}
		g.kds = append(g.kds, l.Items...)
	} else if g.noArgsReact == noArgsNoDeployments {
		// do nothing
	} else if g.noArgsReact == noArgsAllDeployments {
		var l v1beta1.KluctlDeploymentList
		err = g.client.List(ctx, &l, client.InNamespace(g.args.Namespace))
		if err != nil {
			return err
		}
		g.kds = append(g.kds, l.Items...)
	} else if g.noArgsReact == noArgsAutoDetectProject || g.noArgsReact == noArgsAutoDetectProjectAsk {
		err = g.autoDetectDeployment(ctx)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("either name or label selector must be set")
	}

	g.resultStore, err = buildResultStoreRO(ctx, restConfig, mapper, &g.args.CommandResultReadOnlyFlags)
	if err != nil {
		return err
	}

	err = g.initSourceOverrides(ctx)
	if err != nil {
		status.Warningf(ctx, "Failed to initialize source overrides: %s", err.Error())
	}

	return nil
}

func (g *gitopsCmdHelper) collectLocalProjectInfo(ctx context.Context) error {
	projectDir, err := g.args.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}
	gitRoot, err := git.DetectGitRepositoryRoot(projectDir)
	if err != nil {
		return err
	}
	gitInfo, projectKey, err := git.BuildGitInfo(ctx, gitRoot, projectDir)
	if err != nil {
		return err
	}

	if projectKey.RepoKey == (types.RepoKey{}) {
		return fmt.Errorf("failed to determine repo key")
	}

	g.projectGitRoot = gitRoot
	g.projectGitInfo = &gitInfo
	g.projectKey = &projectKey

	return nil
}

func (g *gitopsCmdHelper) autoDetectDeployment(ctx context.Context) error {
	if g.projectKey == nil {
		return fmt.Errorf("auto-detection of KluctlDeployments only possible if local project is a Git repository")
	}

	msg := fmt.Sprintf("Auto-detecting KluctlDeployments via repo key %s", g.projectKey.RepoKey.String())
	if g.projectKey.SubDir != "" {
		msg += fmt.Sprintf(" and sub directory %s", g.projectKey.SubDir)
	}

	st := status.Start(ctx, msg)
	defer st.Failed()

	var l v1beta1.KluctlDeploymentList
	err := g.client.List(ctx, &l, client.InNamespace(g.args.Namespace))
	if err != nil {
		return err
	}

	var matching []v1beta1.KluctlDeployment
	for _, kd := range l.Items {
		isGit := false
		var u, subDir string
		if kd.Spec.Source.Git != nil {
			isGit = true
			u = kd.Spec.Source.Git.URL
			subDir = kd.Spec.Source.Git.Path
		} else if kd.Spec.Source.Oci != nil {
			u = kd.Spec.Source.Oci.URL
			subDir = kd.Spec.Source.Oci.Path
		} else if kd.Spec.Source.URL != nil {
			isGit = true
			u = *kd.Spec.Source.URL
			subDir = kd.Spec.Source.Path
		}
		var repoKey types.RepoKey
		if isGit {
			repoKey, err = types.NewRepoKeyFromGitUrl(u)
		} else {
			repoKey, err = types.NewRepoKeyFromUrl(u)
		}
		if err != nil {
			status.Warningf(ctx, "Failed to determine repo key for KluctlDeployment %s/%s with source url %s: %s", kd.Namespace, kd.Name, u, err.Error())
			continue
		}

		if repoKey != g.projectKey.RepoKey || subDir != g.projectKey.SubDir {
			continue
		}

		matching = append(matching, kd)
	}

	if len(matching) == 0 {
		return fmt.Errorf("no matching KluctlDeployments found")
	}

	if len(matching) == 1 {
		if g.noArgsReact == noArgsAutoDetectProjectAsk {
			kd := matching[0]
			if !prompts.AskForConfirmation(ctx, fmt.Sprintf("Auto-detected %s/%s, do you want to run the command for this deployment?", kd.Namespace, kd.Name)) {
				return fmt.Errorf("aborted")
			}
		}
		g.kds = matching
		st.Success()
		return nil
	}

	if g.noArgsReact == noArgsAutoDetectProjectAsk {
		if len(matching) == 1 {
			if !prompts.AskForConfirmation(ctx, "Auto-detected %s/%s, do you want to run the command for this deployment?") {
				return fmt.Errorf("aborted")
			}
		}

		var choices utils.OrderedMap[string, string]
		for i, kd := range matching {
			choices.Set(fmt.Sprintf("%d", i+1), fmt.Sprintf("%s/%s", kd.Namespace, kd.Name))
		}
		choices.Set("a", "All")

		response, err := prompts.AskForChoice(ctx, "Auto-detected multiple KluctlDeployments. Which one do you want to run the command for?", &choices)
		if err != nil {
			return err
		}

		if response == "a" {
			g.kds = matching
		} else {
			idx, _ := strconv.ParseInt(response, 10, 32)
			g.kds = append(g.kds, matching[idx-1])
		}
	} else {
		g.kds = matching
	}

	st.Success()

	return nil
}

func (g *gitopsCmdHelper) patchDeployment(ctx context.Context, key client.ObjectKey, cb func(kd *v1beta1.KluctlDeployment) error) (*v1beta1.KluctlDeployment, error) {
	s := status.Startf(ctx, "Patching KluctlDeployment %s/%s", key.Namespace, key.Name)
	defer s.Failed()

	var kd v1beta1.KluctlDeployment
	err := g.client.Get(ctx, key, &kd)
	if err != nil {
		return nil, err
	}

	patch := client.MergeFrom(kd.DeepCopy())
	err = cb(&kd)
	if err != nil {
		return nil, err
	}

	err = g.client.Patch(ctx, &kd, patch, client.FieldOwner("kluctl"))
	if err != nil {
		return nil, err
	}

	s.Success()

	return &kd, nil
}

func (g *gitopsCmdHelper) updateDeploymentStatus(ctx context.Context, key client.ObjectKey, cb func(kd *v1beta1.KluctlDeployment) error, retries int) error {
	s := status.Startf(ctx, "Updating KluctlDeployment %s/%s", key.Namespace, key.Name)
	defer s.Failed()

	var lastErr error
	for i := 0; i < retries; i++ {
		var kd v1beta1.KluctlDeployment
		err := g.client.Get(ctx, key, &kd)
		if err != nil {
			return err
		}

		err = cb(&kd)
		if err != nil {
			return err
		}

		err = g.client.Status().Update(ctx, &kd, client.FieldOwner("kluctl"))
		lastErr = err
		if err != nil {
			if !errors.IsConflict(err) {
				return err
			}
		}
		s.Success()
		return nil
	}

	return lastErr
}

func (g *gitopsCmdHelper) patchDeploymentStatus(ctx context.Context, key client.ObjectKey, cb func(kd *v1beta1.KluctlDeployment) error) error {
	var kd v1beta1.KluctlDeployment
	err := g.client.Get(ctx, key, &kd)
	if err != nil {
		return err
	}

	patch := client.MergeFrom(kd.DeepCopy())
	err = cb(&kd)
	if err != nil {
		return err
	}

	err = g.client.Status().Patch(ctx, &kd, patch, client.FieldOwner("kluctl"))
	if err != nil {
		return err
	}

	return nil
}

func (g *gitopsCmdHelper) patchManualRequest(ctx context.Context, key client.ObjectKey, requestAnnotation string, value string) error {
	_, err := g.patchDeployment(ctx, key, func(kd *v1beta1.KluctlDeployment) error {
		overridePatch, err := g.buildOverridePatch(ctx, kd)
		if err != nil {
			return err
		}

		mr := v1beta1.ManualRequest{
			RequestValue: value,
		}
		if len(overridePatch) != 0 && !bytes.Equal(overridePatch, []byte("{}")) {
			mr.OverridesPatch = &runtime.RawExtension{Raw: overridePatch}
		}

		mrJson, err := yaml.WriteJsonString(&mr)
		if err != nil {
			return err
		}

		a := kd.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		a[requestAnnotation] = mrJson

		kd.SetAnnotations(a)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (g *gitopsCmdHelper) buildOverridePatch(ctx context.Context, kdIn *v1beta1.KluctlDeployment) ([]byte, error) {
	cobraCmd := getCobraCommand(ctx)
	if cobraCmd == nil {
		panic("no cobra cmd")
	}

	handleFlag := func(name string, cb func(f *flag.Flag)) {
		f := cobraCmd.Flag(name)
		if f == nil {
			return
		}
		if f.Changed {
			cb(f)
		}
	}

	kd := kdIn.DeepCopy()

	handleFlag("dry-run", func(f *flag.Flag) {
		kd.Spec.DryRun = g.overridableArgs.DryRun
	})
	handleFlag("force-apply", func(f *flag.Flag) {
		kd.Spec.ForceApply = g.overridableArgs.ForceApply
	})
	handleFlag("replace-on-error", func(f *flag.Flag) {
		kd.Spec.ReplaceOnError = g.overridableArgs.ReplaceOnError
	})
	handleFlag("force-replace-on-error", func(f *flag.Flag) {
		kd.Spec.ForceReplaceOnError = g.overridableArgs.ForceReplaceOnError
	})
	handleFlag("abort-on-error", func(f *flag.Flag) {
		kd.Spec.AbortOnError = g.overridableArgs.AbortOnError
	})
	handleFlag("no-wait", func(f *flag.Flag) {
		kd.Spec.NoWait = utils.ParseBoolOrFalse(f.Value.String())
	})
	handleFlag("prune", func(f *flag.Flag) {
		kd.Spec.Prune = utils.ParseBoolOrFalse(f.Value.String())
	})

	if g.overridableArgs.Target != "" {
		kd.Spec.Target = &g.overridableArgs.Target
	}
	if g.overridableArgs.TargetNameOverride != "" {
		kd.Spec.TargetNameOverride = &g.overridableArgs.TargetNameOverride
	}
	if g.overridableArgs.TargetContext != "" {
		kd.Spec.Context = &g.overridableArgs.TargetContext
	}

	fis, err := g.overridableArgs.ImageFlags.LoadFixedImagesFromArgs()
	if err != nil {
		return nil, err
	}
	kd.Spec.Images = append(kd.Spec.Images, fis...)

	inc, err := g.overridableArgs.InclusionFlags.ParseInclusionFromArgs()
	if err != nil {
		return nil, err
	}
	kd.Spec.IncludeTags = append(kd.Spec.IncludeTags, inc.GetIncludes("tag")...)
	kd.Spec.ExcludeTags = append(kd.Spec.ExcludeTags, inc.GetExcludes("tag")...)
	kd.Spec.IncludeDeploymentDirs = append(kd.Spec.IncludeDeploymentDirs, inc.GetIncludes("deploymentItemDir")...)
	kd.Spec.ExcludeDeploymentDirs = append(kd.Spec.ExcludeDeploymentDirs, inc.GetExcludes("deploymentItemDir")...)

	err = g.overrideDeploymentArgs(kd)
	if err != nil {
		return nil, err
	}

	err = g.buildSourceOverrides(ctx, kd)

	kdOrigS, err := yaml.WriteJsonString(kdIn)
	if err != nil {
		return nil, err
	}
	kdS, err := yaml.WriteJsonString(kd)
	if err != nil {
		return nil, err
	}

	patchB, err := json_patch.CreateMergePatch([]byte(kdOrigS), []byte(kdS))
	if err != nil {
		return nil, err
	}

	return patchB, nil
}

func (g *gitopsCmdHelper) overrideDeploymentArgs(kd *v1beta1.KluctlDeployment) error {
	overrideArgs, err := g.overridableArgs.ArgsFlags.LoadArgs()
	if err != nil {
		return err
	}
	if overrideArgs.IsZero() {
		return nil
	}

	newArgs := uo.New()
	if kd.Spec.Args != nil {
		x, err := uo.FromString(string(kd.Spec.Args.Raw))
		if err != nil {
			return err
		}
		newArgs = x
	}
	newArgs.Merge(overrideArgs)
	b, err := yaml.WriteJsonString(newArgs)
	if err != nil {
		return err
	}
	kd.Spec.Args = &runtime.RawExtension{Raw: []byte(b)}

	return nil
}

func (g *gitopsCmdHelper) buildSourceOverrides(ctx context.Context, kd *v1beta1.KluctlDeployment) error {
	var err error
	var proxyUrl *url.URL

	if g.soClient != nil {
		proxyUrl, err = g.soClient.BuildProxyUrl()
		if err != nil {
			return err
		}
	} else if len(g.soResolver.Overrides) != 0 {
		return fmt.Errorf("local source overrides present but no connection to source override proxy established")
	} else {
		status.Warningf(ctx, "Will not try to auto override local project source")
		return nil
	}

	for _, ro := range g.soResolver.Overrides {
		kd.Spec.SourceOverrides = append(kd.Spec.SourceOverrides, v1beta1.SourceOverride{
			RepoKey: ro.RepoKey,
			Url:     proxyUrl.String(),
			IsGroup: ro.IsGroup,
		})
	}

	err = g.buildProjectDirSourceOverride(kd, proxyUrl)
	if err != nil {
		status.Warningf(ctx, "Failed to add source override for local deployment project: %s", err.Error())
	}

	return nil
}

func (g *gitopsCmdHelper) buildProjectDirSourceOverride(kd *v1beta1.KluctlDeployment, proxyUrl *url.URL) error {
	if g.projectKey == nil || proxyUrl == nil {
		return nil
	}

	g.soResolver.Overrides = append(g.soResolver.Overrides, sourceoverride.RepoOverride{
		RepoKey:  g.projectKey.RepoKey,
		Override: g.projectGitRoot,
		IsGroup:  false,
	})

	kd.Spec.SourceOverrides = append(kd.Spec.SourceOverrides, v1beta1.SourceOverride{
		RepoKey: g.projectKey.RepoKey,
		Url:     proxyUrl.String(),
		IsGroup: false,
	})

	return nil
}

func (g *gitopsCmdHelper) waitForRequestToStart(ctx context.Context, key client.ObjectKey, requestValue string, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.ManualRequestResult) (*v1beta1.ManualRequestResult, error) {
	s := status.Startf(ctx, "Waiting for controller to start processing the request")
	defer s.Failed()

	sleep := time.Second * 1

	var rr *v1beta1.ManualRequestResult
	for {
		var kd v1beta1.KluctlDeployment
		err := g.client.Get(ctx, key, &kd)
		if err != nil {
			return nil, err
		}

		rr = getRequestResult(&kd.Status)
		if rr == nil {
			time.Sleep(sleep)
			continue
		}

		if rr.Request.RequestValue != requestValue {
			time.Sleep(sleep)
			continue
		}
		break
	}
	s.Success()
	return rr, nil
}

func (g *gitopsCmdHelper) waitForRequestToFinish(ctx context.Context, key client.ObjectKey, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.ManualRequestResult) (*v1beta1.ManualRequestResult, error) {
	sleep := time.Second * 1

	var rr *v1beta1.ManualRequestResult
	for {
		var kd v1beta1.KluctlDeployment
		err := g.client.Get(ctx, key, &kd)
		if err != nil {
			return nil, err
		}

		rr = getRequestResult(&kd.Status)
		if rr == nil {
			time.Sleep(sleep)
			continue
		}

		if rr.EndTime == nil {
			time.Sleep(sleep)
			continue
		}
		break
	}
	return rr, nil
}

func (g *gitopsCmdHelper) waitForRequestToStartAndFinish(ctx context.Context, key client.ObjectKey, requestValue string, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.ManualRequestResult) (*v1beta1.ManualRequestResult, error) {
	rrStarted, err := g.waitForRequestToStart(ctx, key, requestValue, getRequestResult)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})

	var rrFinished *v1beta1.ManualRequestResult

	gh := utils.NewGoHelper(ctx, 0)
	gh.RunE(func() error {
		defer func() {
			close(stopCh)
		}()
		var err error
		rrFinished, err = g.waitForRequestToFinish(ctx, key, getRequestResult)
		return err
	})
	gh.RunE(func() error {
		return g.watchLogs(ctx, stopCh, key, true, rrStarted.ReconcileId)
	})
	gh.Wait()

	if gh.ErrorOrNil() != nil {
		return nil, gh.ErrorOrNil()
	}

	return rrFinished, nil
}

func (g *gitopsCmdHelper) watchLogs(ctx context.Context, stopCh chan struct{}, key client.ObjectKey, follow bool, reconcileId string) error {
	if key == (client.ObjectKey{}) {
		status.Infof(ctx, "Watching logs...")
	} else {
		status.Infof(ctx, "Watching logs for %s/%s...", key.Namespace, key.Name)
	}

	logsCh, err := logs.WatchControllerLogs(ctx, g.corev1Client, g.args.ControllerNamespace, key, reconcileId, g.logsArgs.LogSince, follow)
	if err != nil {
		return err
	}

	timeout := g.logsArgs.LogGroupingTime
	timeoutCh := time.After(timeout)
	for {
		select {
		case <-ctx.Done():
			g.flushLogLines(ctx, true)
			return ctx.Err()
		case <-stopCh:
			g.flushLogLines(ctx, true)
			return nil
		case <-timeoutCh:
			g.flushLogLines(ctx, true)
			timeoutCh = time.After(timeout)
		case l, ok := <-logsCh:
			if !ok {
				g.flushLogLines(ctx, true)
				return nil
			}
			g.handleLogLine(ctx, l)
		}
	}
}

func (g *gitopsCmdHelper) handleLogLine(ctx context.Context, logLine logs.LogLine) {
	g.logsMutex.Lock()
	defer g.logsMutex.Unlock()

	lk := logsKey{
		objectKey:   client.ObjectKey{Name: logLine.Name, Namespace: logLine.Namespace},
		reconcileId: logLine.ReconcileID,
	}

	lb := g.logsBufs[lk]
	if lb == nil {
		lb = &logsBuf{}
		g.logsBufs[lk] = lb
	}
	lb.lines = append(lb.lines, logLine)

	hasOther := false
	for k, x := range g.logsBufs {
		if k != lk && len(x.lines) != 0 {
			hasOther = true
			break
		}
	}
	if !hasOther && (g.lastFlushedLogsKey == nil || *g.lastFlushedLogsKey == lk) {
		g.flushLogLines(ctx, false)
	}
}

func (g *gitopsCmdHelper) flushLogLines(ctx context.Context, needLock bool) {
	if needLock {
		g.logsMutex.Lock()
		defer g.logsMutex.Unlock()
	}

	for lk, lb := range g.logsBufs {
		lk := lk
		if len(lb.lines) == 0 {
			if time.Now().After(lb.lastFlushTime.Add(5 * time.Second)) {
				delete(g.logsBufs, lk)
			}
			continue
		}
		if g.lastFlushedLogsKey == nil || *g.lastFlushedLogsKey != lk {
			g.lastFlushedLogsKey = &lk
			lb.lastFlushTime = time.Now()
			if lk.reconcileId == "" {
				status.Info(ctx, "Showing general controller logs")
			} else {
				status.Infof(ctx, "Showing logs for %s/%s and reconciliation ID %s", lk.objectKey.Namespace, lk.objectKey.Name, lk.reconcileId)
			}
			status.Flush(ctx)
		}

		for _, l := range lb.lines {
			var s string
			if g.logsArgs.LogTime {
				s = fmt.Sprintf("%s: %s\n", l.Timestamp.Format(time.RFC3339), s)
			} else {
				s = l.Msg + "\n"
			}
			_, _ = getStdout(ctx).WriteString(s)
		}
		lb.lines = lb.lines[0:0]
	}
}

func (g *gitopsCmdHelper) initSourceOverrides(ctx context.Context) error {
	var err error
	g.soResolver, err = g.overridableArgs.SourceOverrides.ParseOverrides(ctx)
	if err != nil {
		return err
	}

	pods, err := g.corev1Client.Pods(g.args.ControllerNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=kluctl-controller",
	})
	if err != nil {
		return err
	}
	var pod *v12.Pod
	if len(pods.Items) > 0 {
		pod = &pods.Items[rand.Int()%len(pods.Items)]
	}

	soClient, err := sourceoverride.NewClientCli(ctx, g.client, g.args.ControllerNamespace, g.soResolver)
	if err != nil {
		return err
	}

	if pod != nil {
		err = soClient.ConnectToPod(g.restConfig, *pod)
		if err != nil {
			return err
		}
	} else {
		if g.args.LocalSourceOverridePort == 0 {
			return nil
		}
		err = soClient.Connect(fmt.Sprintf("localhost:%d", g.args.LocalSourceOverridePort))
		if err != nil {
			return err
		}
	}
	g.soClient = soClient

	err = g.soClient.Handshake()
	if err != nil {
		return err
	}

	go func() {
		err := soClient.Start()
		if err != nil {
			status.Error(ctx, err.Error())
		}
	}()

	return nil
}

func (g *gitopsCmdHelper) checkCRDSupport(ctx context.Context) error {
	var crd apiextensionsv1.CustomResourceDefinition
	err := g.client.Get(ctx, client.ObjectKey{Name: "kluctldeployments.gitops.kluctl.io"}, &crd)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("KluctlDeployment CRD not found, which usually means the kluctl-controller is not installed")
		}
		return err
	}

	for _, version := range crd.Spec.Versions {
		if !version.Storage {
			continue
		}
		status := version.Schema.OpenAPIV3Schema.Properties["status"]
		if _, ok := status.Properties["reconcileRequestResult"]; ok {
			// seems to be recent enough
			return nil
		}
	}

	return fmt.Errorf("the KluctlDeployment CRD on the cluster seems to be outdated. Ensure to install at least kluctl-controller v2.22.0 with its corresponding CRDs")
}

func init() {
	utilruntime.Must(v1beta1.AddToScheme(scheme.Scheme))
}
