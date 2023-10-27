package commands

import (
	"bytes"
	"context"
	"fmt"
	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	flag "github.com/spf13/pflag"
	v12 "k8s.io/api/core/v1"
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
)

func (g *gitopsCmdHelper) init(ctx context.Context) error {
	s := status.Startf(ctx, "Initializing")
	defer s.Failed()

	g.logsBufs = map[logsKey]*logsBuf{}

	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: g.args.Context,
	}
	var err error
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
		err = g.client.List(ctx, &l)
		if err != nil {
			return err
		}
		g.kds = append(g.kds, l.Items...)
	} else {
		return fmt.Errorf("either name or label selector must be set")
	}

	g.resultStore, err = buildResultStoreRO(ctx, restConfig, mapper, &g.args.CommandResultReadOnlyFlags)
	if err != nil {
		return err
	}

	err = g.initSourceOverrides(ctx)
	if err != nil {
		return err
	}

	s.Success()

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
		a := kd.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		a[requestAnnotation] = value

		onetimePatch, err := g.buildOnetimePatch(ctx, kd)
		if err != nil {
			return err
		}
		if len(onetimePatch) != 0 && !bytes.Equal(onetimePatch, []byte("{}")) {
			a[v1beta1.KluctlOnetimePatchAnnotation] = string(onetimePatch)
		} else {
			delete(a, v1beta1.KluctlOnetimePatchAnnotation)
		}

		kd.SetAnnotations(a)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (g *gitopsCmdHelper) buildOnetimePatch(ctx context.Context, kdIn *v1beta1.KluctlDeployment) ([]byte, error) {
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

	var proxyUrl *url.URL
	if len(g.soResolver.Overrides) != 0 {
		if g.soClient == nil {
			return nil, fmt.Errorf("local source overrides present but no connection to source override proxy established")
		}
		proxyUrl, err = g.soClient.BuildProxyUrl()
		if err != nil {
			return nil, err
		}
	}
	for _, ro := range g.soResolver.Overrides {
		kd.Spec.SourceOverrides = append(kd.Spec.SourceOverrides, v1beta1.SourceOverride{
			RepoKey: ro.RepoKey,
			Url:     proxyUrl.String(),
			IsGroup: ro.IsGroup,
		})
	}

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

		if rr.RequestValue != requestValue {
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

	logsCh, err := logs.WatchControllerLogs(ctx, g.corev1Client, "kluctl-system", key, reconcileId, g.logsArgs.LogSince, follow)
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

	if len(g.soResolver.Overrides) == 0 {
		return nil
	}

	controllerNamespace := "kluctl-system"
	pods, err := g.corev1Client.Pods(controllerNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=kluctl-controller",
	})
	if err != nil {
		return err
	}
	var pod *v12.Pod
	if len(pods.Items) > 0 {
		pod = &pods.Items[rand.Int()%len(pods.Items)]
	}

	soClient, err := sourceoverride.NewClientCli(ctx, g.soResolver)
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

func init() {
	utilruntime.Must(v1beta1.AddToScheme(scheme.Scheme))
}
