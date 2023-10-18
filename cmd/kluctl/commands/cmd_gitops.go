package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

type gitopsCmd struct {
	Reconcile gitopsReconcileCmd `cmd:"" help:"Trigger a GitOps reconciliation"`
	Deploy    gitopsDeployCmd    `cmd:"" help:"Trigger a GitOps deployment"`
	Prune     gitopsPruneCmd     `cmd:"" help:"Trigger a GitOps prune"`
	Validate  gitopsValidateCmd  `cmd:"" help:"Trigger a GitOps validate"`
	Logs      gitopsLogsCmd      `cmd:"" help:"Show logs from controller"`
}

type gitopsCmdHelper struct {
	args     args.GitOpsArgs
	logsArgs args.GitOpsLogArgs

	kds []v1beta1.KluctlDeployment

	client       client.Client
	corev1Client *v1.CoreV1Client

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

func (g *gitopsCmdHelper) init(ctx context.Context, allowNoArgs bool) error {
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

	g.client, err = client.NewWithWatch(restConfig, client.Options{})
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

	var opts []client.ListOption
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
	} else if !allowNoArgs {
		return fmt.Errorf("either name or label selector must be set")
	}

	g.resultStore, err = buildResultStore(ctx, restConfig, g.args.CommandResultFlags, false)
	if err != nil {
		return err
	}

	return nil
}

func (g *gitopsCmdHelper) patchAnnotation(ctx context.Context, kd *v1beta1.KluctlDeployment, name string, value string) error {
	s := status.Startf(ctx, "Patching KluctlDeployment %s/%s with annotation %s=%s", kd.Namespace, kd.Name, name, value)
	defer s.Failed()

	patch := client.MergeFrom(kd.DeepCopy())

	a := kd.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[name] = value
	kd.SetAnnotations(a)

	err := g.client.Patch(ctx, kd, patch, client.FieldOwner("kluctl"))
	if err != nil {
		return err
	}

	s.Success()

	return nil
}

func (g *gitopsCmdHelper) waitForRequestToStart(ctx context.Context, key client.ObjectKey, requestValue string, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult) (*v1beta1.RequestResult, error) {
	s := status.Startf(ctx, "Waiting for controller to start processing the request")
	defer s.Failed()

	sleep := time.Second * 1

	var rr *v1beta1.RequestResult
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

func (g *gitopsCmdHelper) waitForRequestToFinish(ctx context.Context, key client.ObjectKey, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult) (*v1beta1.RequestResult, error) {
	sleep := time.Second * 1

	var rr *v1beta1.RequestResult
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

func (g *gitopsCmdHelper) waitForRequestToStartAndFinish(ctx context.Context, key client.ObjectKey, requestValue string, getRequestResult func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult) (*v1beta1.RequestResult, error) {
	rr, err := g.waitForRequestToStart(ctx, key, requestValue, getRequestResult)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})

	gh := utils.NewGoHelper(ctx, 0)
	gh.RunE(func() error {
		defer func() {
			close(stopCh)
		}()
		var err error
		rr, err = g.waitForRequestToFinish(ctx, key, getRequestResult)
		return err
	})
	gh.RunE(func() error {
		return g.watchLogs(ctx, stopCh, key, true, "")
	})
	gh.Wait()

	if gh.ErrorOrNil() != nil {
		return nil, gh.ErrorOrNil()
	}
	return rr, nil
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

func init() {
	utilruntime.Must(v1beta1.AddToScheme(scheme.Scheme))
}
