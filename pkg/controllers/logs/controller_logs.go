package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync/atomic"
	"time"
)

type LogLine struct {
	Level       string    `json:"level"`
	Timestamp   time.Time `json:"ts"`
	Msg         string    `json:"msg"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	ReconcileID string    `json:"reconcileID"`
}

func WatchControllerLogs(ctx context.Context, c *v1.CoreV1Client, controllerNamespace string, kdKey client.ObjectKey, reconcileId string, since time.Duration, follow bool) (chan LogLine, error) {
	pods, err := c.Pods(controllerNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=kluctl-controller",
	})
	if err != nil {
		return nil, err
	}

	var streams []io.ReadCloser
	cleanupStreams := func() {
		for _, s := range streams {
			_ = s.Close()
		}
	}

	ok := false
	defer func() {
		if ok {
			return
		}
		cleanupStreams()
	}()

	for _, pod := range pods.Items {
		since2 := int64(since.Seconds())
		rc, err := c.Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow:       follow,
			SinceSeconds: &since2,
		}).Stream(ctx)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		streams = append(streams, rc)
	}

	var closedCount atomic.Int32
	rawLineCh := make(chan string)
	for _, s := range streams {
		s := s
		go func() {
			sc := bufio.NewScanner(s)
			for sc.Scan() {
				l := sc.Text()
				rawLineCh <- l
			}
			closedCount.Add(1)
			if closedCount.Load() == int32(len(streams)) {
				close(rawLineCh)
			}
		}()
	}

	ok = true

	logCh := make(chan LogLine)
	go func() {
		defer cleanupStreams()
		for {
			select {
			case <-ctx.Done():
				close(logCh)
				return
			case l, ok := <-rawLineCh:
				if !ok {
					close(logCh)
					return
				}
				var ll LogLine
				err = json.Unmarshal([]byte(l), &ll)
				if err != nil {
					continue
				}

				if kdKey.Name != "" && ll.Name != kdKey.Name {
					continue
				}
				if kdKey.Namespace != "" && ll.Namespace != kdKey.Namespace {
					continue
				}

				if reconcileId != "" && ll.ReconcileID != reconcileId {
					continue
				}

				logCh <- ll
			}
		}
	}()

	return logCh, nil
}
