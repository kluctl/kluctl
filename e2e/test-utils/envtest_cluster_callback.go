package test_utils

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"time"
)

type CallbackHandler func(request admission.Request)
type CallbackHandlerEntry struct {
	GVR      schema.GroupVersionResource
	Callback CallbackHandler
}

func (k *EnvTestCluster) buildServeCallback() http.Handler {
	wh := &webhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, request admission.Request) admission.Response {
			k.handleWebhook(request)
			return admission.Allowed("")
		}),
	}
	wh.LogConstructor = func(base logr.Logger, req *admission.Request) logr.Logger {
		return logr.New(log.NullLogSink{})
	}
	return wh
}

func (k *EnvTestCluster) startCallbackServer() error {
	k.callbackServer = webhook.NewServer(webhook.Options{
		Host:    k.env.WebhookInstallOptions.LocalServingHost,
		Port:    k.env.WebhookInstallOptions.LocalServingPort,
		CertDir: k.env.WebhookInstallOptions.LocalServingCertDir,
	})

	for _, vwh := range k.env.WebhookInstallOptions.ValidatingWebhooks {
		path := "/" + vwh.Name
		k.callbackServer.Register(path, k.buildServeCallback())
	}

	ctx, cancel := context.WithCancel(context.Background())
	k.callbackServerStop = cancel

	go func() {
		_ = k.callbackServer.Start(ctx)
	}()

	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(k.env.WebhookInstallOptions.LocalServingHost, fmt.Sprintf("%d", k.env.WebhookInstallOptions.LocalServingPort)))
	if err != nil {
		return err
	}

	endTime := time.Now().Add(time.Second * 10)
	for true {
		if time.Now().After(endTime) {
			return fmt.Errorf("timeout while waiting for webhook server")
		}
		c, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_ = c.Close()
		break
	}

	return nil
}

func (k *EnvTestCluster) InitWebhookCallback(gvr schema.GroupVersionResource, isNamespaced bool) {
	scope := admissionv1.ClusterScope
	if isNamespaced {
		scope = admissionv1.NamespacedScope
	}

	failedTypeV1 := admissionv1.Fail
	equivalentTypeV1 := admissionv1.Equivalent
	noSideEffectsV1 := admissionv1.SideEffectClassNone
	timeout := int32(30)

	group := gvr.Group
	if gvr.Group == "" {
		group = "none"
	}
	name := fmt.Sprintf("%s-%s-%s-callback", group, gvr.Version, gvr.Resource)

	k.env.WebhookInstallOptions.ValidatingWebhooks = append(k.env.WebhookInstallOptions.ValidatingWebhooks, &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingWebhookConfiguration",
			APIVersion: "admissionregistration.k8s.io/v1",
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: fmt.Sprintf("%s.callback.kubectl.io", name),
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{"CREATE", "UPDATE"},
						Rule: admissionv1.Rule{
							APIGroups:   []string{gvr.Group},
							APIVersions: []string{gvr.Version},
							Resources:   []string{gvr.Resource},
							Scope:       &scope,
						},
					},
				},
				FailurePolicy: &failedTypeV1,
				MatchPolicy:   &equivalentTypeV1,
				SideEffects:   &noSideEffectsV1,
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path: &name,
					},
				},
				AdmissionReviewVersions: []string{"v1"},
				TimeoutSeconds:          &timeout,
			},
		},
	})
}

func (k *EnvTestCluster) handleWebhook(request admission.Request) {
	k.webhookHandlersMutex.Lock()
	defer k.webhookHandlersMutex.Unlock()

	for _, e := range k.webhookHandlers {
		if e.GVR.Group == request.Resource.Group && e.GVR.Version == request.Resource.Version && e.GVR.Resource == request.Resource.Resource {
			e.Callback(request)
		}
	}
}

func (k *EnvTestCluster) AddWebhookHandler(gvr schema.GroupVersionResource, cb CallbackHandler) *CallbackHandlerEntry {
	k.webhookHandlersMutex.Lock()
	defer k.webhookHandlersMutex.Unlock()

	entry := &CallbackHandlerEntry{
		GVR:      gvr,
		Callback: cb,
	}
	k.webhookHandlers = append(k.webhookHandlers, entry)

	return entry
}

func (k *EnvTestCluster) RemoveWebhookHandler(e *CallbackHandlerEntry) {
	k.webhookHandlersMutex.Lock()
	defer k.webhookHandlersMutex.Unlock()
	old := k.webhookHandlers
	k.webhookHandlers = make([]*CallbackHandlerEntry, 0, len(k.webhookHandlers))
	for _, e2 := range old {
		if e != e2 {
			k.webhookHandlers = append(k.webhookHandlers, e2)
		}
	}
}
