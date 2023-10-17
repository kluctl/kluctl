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
	"time"
)

func WatchControllerLogs(ctx context.Context, c *v1.CoreV1Client, controllerNamespace string, kdKey client.ObjectKey, reconcileId string, since time.Duration) (chan string, error) {
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
			Follow:       true,
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

	rawLineCh := make(chan string)
	for _, s := range streams {
		s := s
		go func() {
			sc := bufio.NewScanner(s)
			for sc.Scan() {
				l := sc.Text()
				rawLineCh <- l
			}
		}()
	}

	ok = true

	logCh := make(chan string)
	go func() {
		defer cleanupStreams()
		for {
			select {
			case <-ctx.Done():
				return
			case l := <-rawLineCh:
				var j map[string]any
				err = json.Unmarshal([]byte(l), &j)
				if err != nil {
					continue
				}

				name := j["name"]
				namespace := j["namespace"]

				if kdKey.Name != "" && name != kdKey.Name {
					continue
				}
				if kdKey.Namespace != "" && namespace != kdKey.Namespace {
					continue
				}

				rid := j["reconcileID"]
				if reconcileId != "" && rid != reconcileId {
					continue
				}

				logCh <- l
			}
		}
	}()

	return logCh, nil
}
