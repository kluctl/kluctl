package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 300
	interval = time.Second * 5
)

func startKluctlController(t *testing.T) context.CancelFunc {
	ctx, ctxCancel := context.WithCancel(context.Background())
	args := []string{
		"controller",
	}
	done := make(chan struct{})
	go func() {
		_, _, err := test_utils.KluctlExecute(t, ctx, args...)
		if err != nil {
			t.Error(err)
		}
		close(done)
	}()

	cancel := func() {
		ctxCancel()
		<-done
	}

	t.Cleanup(cancel)
	return cancel
}

func triggerReconcile(t *testing.T, k *test_utils.EnvTestCluster, key client.ObjectKey) string {
	reconcileId := fmt.Sprintf("%d", rand.Int63())

	updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		a := kd.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		a[kluctlv1.KluctlRequestReconcileAnnotation] = reconcileId
		kd.SetAnnotations(a)
	})
	return reconcileId
}

func waitForReconcile(t *testing.T, k *test_utils.EnvTestCluster, key client.ObjectKey) {
	g := gomega.NewWithT(t)

	reconcileId := triggerReconcile(t, k, key)

	g.Eventually(func() bool {
		var kd kluctlv1.KluctlDeployment
		err := k.Client.Get(context.TODO(), key, &kd)
		g.Expect(err).To(Succeed())
		return kd.Status.LastHandledReconcileAt == reconcileId
	}, timeout, time.Second).Should(BeTrue())
}

func waitForCommit(t *testing.T, k *test_utils.EnvTestCluster, key client.ObjectKey, commit string) {
	g := gomega.NewWithT(t)

	reconcileId := triggerReconcile(t, k, key)

	g.Eventually(func() bool {
		var kd kluctlv1.KluctlDeployment
		_ = k.Client.Get(context.Background(), key, &kd)
		return kd.Status.LastHandledReconcileAt == reconcileId && kd.Status.ObservedCommit == commit
	}, timeout, time.Second).Should(BeTrue())
}

func createKluctlDeployment(t *testing.T, p *test_utils.TestProject, k *test_utils.EnvTestCluster, target string, args map[string]any) client.ObjectKey {
	gitopsNs := p.TestSlug() + "-gitops"
	createNamespace(t, k, gitopsNs)

	jargs, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}

	kluctlDeployment := &kluctlv1.KluctlDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.TestSlug(),
			Namespace: gitopsNs,
		},
		Spec: kluctlv1.KluctlDeploymentSpec{
			Interval: metav1.Duration{Duration: interval},
			Timeout:  &metav1.Duration{Duration: timeout},
			Target:   &target,
			Args: runtime.RawExtension{
				Raw: jargs,
			},
			Source: kluctlv1.ProjectSource{
				URL: *types2.ParseGitUrlMust(p.GitUrl()),
			},
		},
	}

	err = k.Client.Create(context.Background(), kluctlDeployment)
	if err != nil {
		t.Fatal(err)
	}

	return client.ObjectKeyFromObject(kluctlDeployment)
}

func updateKluctlDeployment(t *testing.T, k *test_utils.EnvTestCluster, key client.ObjectKey, update func(kd *kluctlv1.KluctlDeployment)) *kluctlv1.KluctlDeployment {
	g := NewWithT(t)

	var kd kluctlv1.KluctlDeployment
	err := k.Client.Get(context.TODO(), key, &kd)
	g.Expect(err).To(Succeed())

	patch := client.MergeFrom(kd.DeepCopy())

	update(&kd)

	err = k.Client.Patch(context.TODO(), &kd, patch, client.FieldOwner("kubectl"))
	g.Expect(err).To(Succeed())

	return &kd
}

func TestGitOpsFieldManager(t *testing.T) {
	g := NewWithT(t)

	startKluctlController(t)

	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: v1
  k2: "{{ args.k2 + 1 }}"
`)},
	}, nil)

	key := createKluctlDeployment(t, p, k, "target1", map[string]any{
		"namespace": p.TestSlug(),
		"k2":        42,
	})

	t.Run("initial deployment", func(t *testing.T) {
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.DeployInterval = &kluctlv1.SafeDuration{Duration: metav1.Duration{Duration: interval}}
	})

	cm := &corev1.ConfigMap{}

	t.Run("cm1 is deployed", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("k1", "v1"))
		g.Expect(cm.Data).To(HaveKeyWithValue("k2", "43"))
	})

	t.Run("cm1 is modified and restored", func(t *testing.T) {
		cm.Data["k1"] = "v2"
		err := k.Client.Update(context.TODO(), cm, client.FieldOwner("kubectl"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())
	})

	t.Run("cm1 gets a key added which is not modified by the controller", func(t *testing.T) {
		cm.Data["k1"] = "v2"
		cm.Data["k3"] = "v3"
		err := k.Client.Update(context.TODO(), cm, client.FieldOwner("kubectl"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(cm.Data).To(HaveKeyWithValue("k3", "v3"))
	})

	t.Run("cm1 gets modified with another field manager", func(t *testing.T) {
		patch := client.MergeFrom(cm.DeepCopy())
		cm.Data["k1"] = "v2"

		err := k.Client.Patch(context.TODO(), cm, patch, client.FieldOwner("test-field-manager"))
		g.Expect(err).To(Succeed())

		for i := 0; i < 2; i++ {
			waitForReconcile(t, k, key)
		}

		err = k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("k1", "v2"))
	})

	updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.ForceApply = true
	})

	t.Run("forceApply is true and cm1 gets restored even with another field manager", func(t *testing.T) {
		patch := client.MergeFrom(cm.DeepCopy())
		cm.Data["k1"] = "v2"

		err := k.Client.Patch(context.TODO(), cm, patch, client.FieldOwner("test-field-manager"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())
	})
}

func TestKluctlDeploymentReconciler_Helm(t *testing.T) {
	g := NewWithT(t)
	startKluctlController(t)

	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("target1", nil)

	repoUrl := test_utils.CreateHelmRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}, "", "")
	repoUrlWithCreds := test_utils.CreateHelmRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart2", Version: "0.1.0"},
	}, "test-user", "test-password")
	ociRepoUrlWithCreds := test_utils.CreateOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart3", Version: "0.1.0"},
	}, "test-user", "test-password")

	p.AddHelmDeployment("d1", repoUrl, "test-chart1", "0.1.0", "test-helm-1", p.TestSlug(), nil)
	p.UpdateYaml("d1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	key := createKluctlDeployment(t, p, k, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	waitForCommit(t, k, key, getHeadRevision(t, p))

	cm := &corev1.ConfigMap{}

	t.Run("chart got deployed", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "test-helm-1-test-chart1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("a", "v1"))
	})

	p.AddHelmDeployment("d2", repoUrlWithCreds, "test-chart2", "0.1.0", "test-helm-2", p.TestSlug(), nil)
	p.UpdateYaml("d2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	kd := &kluctlv1.KluctlDeployment{}

	t.Run("chart with credentials fails with 401", func(t *testing.T) {
		g.Eventually(func() bool {
			err := k.Client.Get(context.TODO(), key, kd)
			g.Expect(err).To(Succeed())
			for _, c := range kd.Status.Conditions {
				_ = c
				if c.Type == "Ready" && c.Reason == "PrepareFailed" && strings.Contains(c.Message, "401 Unauthorized") {
					return true
				}
			}
			return false
		}, timeout, time.Second).Should(BeTrue())
	})

	credsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      "helm-secrets-1",
		},
		Data: map[string][]byte{
			"url":      []byte(repoUrlWithCreds),
			"username": []byte("test-user"),
			"password": []byte("test-password"),
		},
	}
	err := k.Client.Create(context.TODO(), credsSecret)
	g.Expect(err).To(Succeed())

	updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.HelmCredentials = append(kd.Spec.HelmCredentials, kluctlv1.HelmCredentials{SecretRef: kluctlv1.LocalObjectReference{Name: "helm-secrets-1"}})
	})

	t.Run("chart with credentials succeeds", func(t *testing.T) {
		g.Eventually(func() bool {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "test-helm-2-test-chart2",
				Namespace: p.TestSlug(),
			}, cm)
			if err != nil {
				return false
			}
			g.Expect(cm.Data).To(HaveKeyWithValue("a", "v1"))
			return true
		}, timeout, time.Second).Should(BeTrue())
	})

	p.AddHelmDeployment("d3", ociRepoUrlWithCreds, "test-chart3", "0.1.0", "test-helm-3", p.TestSlug(), nil)
	p.UpdateYaml("d3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	t.Run("OCI chart with credentials fails with 401", func(t *testing.T) {
		g.Eventually(func() bool {
			err = k.Client.Get(context.TODO(), key, kd)
			g.Expect(err).To(Succeed())
			for _, c := range kd.Status.Conditions {
				_ = c
				if c.Type == "Ready" && c.Reason == "PrepareFailed" && strings.Contains(c.Message, "401 Unauthorized") {
					return true
				}
			}
			return false
		}, timeout, time.Second).Should(BeTrue())
	})

	/*
		TODO enable this when Kluctl supports OCI authentication
		url, err := url2.Parse(ociRepoUrlWithCreds)
		g.Expect(err).To(Succeed())

		dockerJson := map[string]any{
			"auths": map[string]any{
				url.Host: map[string]any{
					"username": "test-user",
					"password": "test-password,",
					"auth":     base64.StdEncoding.EncodeToString([]byte("test-user:test-password")),
				},
			},
		}
		dockerJsonStr, err := json.Marshal(dockerJson)
		g.Expect(err).To(Succeed())

		credsSecret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "helm-secrets-2",
			},
			Data: map[string][]byte{
				"url":               []byte(ociRepoUrlWithCreds),
				".dockerconfigjson": dockerJsonStr,
			},
		}
		err = k.Client.Create(context.TODO(), credsSecret2)
		g.Expect(err).To(Succeed())

		kluctlDeployment.Spec.HelmCredentials = append(kluctlDeployment.Spec.HelmCredentials, flux_utils.LocalObjectReference{Name: "helm-secrets-2"})
		err = k.Client.Update(context.TODO(), kluctlDeployment)
		g.Expect(err).To(Succeed())

		t.Run("OCI chart with credentials succeeds", func(t *testing.T) {
			g.Eventually(func() bool {
				err := k.Client.Get(context.TODO(), client.ObjectKey{
					Name:      "test-helm-3-test-chart3",
					Namespace: namespace,
				}, cm)
				if err != nil {
					return false
				}
				g.Expect(cm.Data).To(HaveKeyWithValue("a", "v1"))
				return true
			}, timeout, time.Second).Should(BeTrue())
		})*/
}

func TestKluctlDeploymentReconciler_Prune(t *testing.T) {
	g := NewWithT(t)
	startKluctlController(t)

	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("target1", nil)

	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: v1
`)},
	}, nil)
	p.AddKustomizeDeployment("d2", []test_utils.KustomizeResource{
		{Name: "cm2.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: "{{ args.namespace }}"
data:
  k1: v1
`)},
	}, nil)

	key := createKluctlDeployment(t, p, k, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	waitForCommit(t, k, key, getHeadRevision(t, p))

	cm := &corev1.ConfigMap{}

	t.Run("cm1 and cm2 got deployed", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.RemoveNestedField("deployments", 1)
		return nil
	})

	g.Eventually(func() bool {
		var obj kluctlv1.KluctlDeployment
		_ = k.Client.Get(context.Background(), key, &obj)
		if obj.Status.LastDeployResult == nil {
			return false
		}
		return obj.Status.LastDeployResult.GitInfo.Commit == getHeadRevision(t, p)
	}, timeout, time.Second).Should(BeTrue())

	t.Run("cm1 and cm2 were not deleted", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Prune = true
	})

	waitForReconcile(t, k, key)

	t.Run("cm1 did not get deleted and cm2 got deleted", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(MatchError("configmaps \"cm2\" not found"))
	})
}

func doTestDelete(t *testing.T, delete bool) {
	g := NewWithT(t)
	startKluctlController(t)

	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("target1", nil)

	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: v1
`)},
	}, nil)

	key := createKluctlDeployment(t, p, k, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	kd := updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Delete = delete
	})

	waitForCommit(t, k, key, getHeadRevision(t, p))

	cm := &corev1.ConfigMap{}

	t.Run("cm1 got deployed", func(t *testing.T) {
		err := k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	g.Expect(k.Client.Delete(context.TODO(), kd)).To(Succeed())

	g.Eventually(func() bool {
		var obj kluctlv1.KluctlDeployment
		err := k.Client.Get(context.Background(), key, &obj)
		if err == nil {
			return false
		}
		if !errors.IsNotFound(err) {
			return false
		}
		return true
	}, timeout, time.Second).Should(BeTrue())

	if delete {
		t.Run("cm1 was deleted", func(t *testing.T) {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(MatchError("configmaps \"cm1\" not found"))
		})
	} else {
		t.Run("cm1 was not deleted", func(t *testing.T) {
			err := k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
		})
	}
}

func TestKluctlDeploymentReconciler_Delete_True(t *testing.T) {
	doTestDelete(t, true)
}

func TestKluctlDeploymentReconciler_Delete_False(t *testing.T) {
	doTestDelete(t, false)
}
