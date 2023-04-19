package vars

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git2 "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/aws"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"github.com/stretchr/testify/assert"
	"go.mozilla.org/sops/v3/age"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func newRP(t *testing.T) *repocache.GitRepoCache {
	grc := repocache.NewGitRepoCache(context.TODO(), &ssh_pool.SshPool{}, auth.NewDefaultAuthProviders("KLUCTL_GIT", nil), nil, 0)
	t.Cleanup(func() {
		grc.Clear()
	})
	return grc
}

func testVarsLoader(t *testing.T, test func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory), objects ...runtime.Object) {
	k, err := k8s.NewK8sCluster(context.TODO(), k8s.NewFakeClientFactory(objects...), false)
	if err != nil {
		t.Fatal(err)
	}
	grc := newRP(t)
	fakeAws := aws.NewFakeClientFactory()

	d := decryptor.NewDecryptor("", decryptor.MaxEncryptedFileSize)
	d.AddLocalKeyService()

	vl := NewVarsLoader(context.TODO(), k, d, grc, fakeAws)
	vc := NewVarsCtx(newJinja2Must(t))

	test(vl, vc, fakeAws)
}

func TestVarsLoader_Values(t *testing.T) {
	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": {"test2": 42}}`),
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_ValuesNoOverrides(t *testing.T) {
	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": {"test2": 42}}`),
		}, nil, "")
		assert.NoError(t, err)

		b := true
		err = vl.LoadVars(vc, &types.VarsSource{
			Values:     uo.FromStringMust(`{"test1": {"test2": 43}}`),
			NoOverride: &b,
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_When(t *testing.T) {
	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "a"}`),
		}, nil, "")
		assert.NoError(t, err)
		err = vl.LoadVars(vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "b"}`),
			When:   `test1 == "b"`,
		}, nil, "")
		assert.NoError(t, err)
		v, _, _ := vc.Vars.GetNestedString("test1")
		assert.Equal(t, "a", v)

		err = vl.LoadVars(vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "b"}`),
			When:   `test1 == "a"`,
		}, nil, "")
		assert.NoError(t, err)
		v, _, _ = vc.Vars.GetNestedString("test1")
		assert.Equal(t, "b", v)
	})
}

func TestVarsLoader_File(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": 42}}`), 0o600)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			File: utils.StrPtr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			File:          utils.StrPtr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			File:          utils.StrPtr("test-missing.yaml"),
		}, []string{d}, "")
		assert.NoError(t, err)
	})
}

func TestVarsLoader_SopsFile(t *testing.T) {
	d := t.TempDir()
	f, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), f, 0o600)

	t.Setenv(age.SopsAgeKeyEnv, string(key))

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			File: utils.StrPtr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_FileWithLoad(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{ load_template("test2.txt") }}}}`), 0o600)
	_ = os.WriteFile(filepath.Join(d, "test2.txt"), []byte(`42`), 0o600)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			File: utils.StrPtr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_FileWithLoadSubDir(t *testing.T) {
	d := t.TempDir()
	_ = os.Mkdir(filepath.Join(d, "subdir"), 0o700)
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{ load_template("test2.txt") }}}}`), 0o600)
	_ = os.WriteFile(filepath.Join(d, "subdir/test2.txt"), []byte(`42`), 0o600)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			File: utils.StrPtr("test.yaml"),
		}, []string{d, filepath.Join(d, "subdir")}, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_FileWithLoadNotExists(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{load_template("test3.txt")}}}}`), 0o600)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			File: utils.StrPtr("test.yaml"),
		}, []string{d}, "")
		assert.EqualError(t, err, "failed to render vars file test.yaml: template test3.txt not found")
	})
}

func TestVarsLoader_Git(t *testing.T) {
	gs := git.NewTestGitServer(t)
	gs.GitInit("repo")
	gs.UpdateYaml("repo", "test.yaml", func(o map[string]any) error {
		o["test1"] = map[string]any{
			"test2": 42,
		}
		return nil
	}, "")

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		url, _ := git_url.Parse(gs.GitRepoUrl("repo"))
		err := vl.LoadVars(vc, &types.VarsSource{
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test.yaml",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		url, _ := git_url.Parse(gs.GitRepoUrl("repo"))
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test-missing.yaml",
			},
		}, nil, "")
		assert.NoError(t, err)
	})
}

func TestVarsLoader_GitBranch(t *testing.T) {
	gs := git.NewTestGitServer(t)
	gs.GitInit("repo")

	wt := gs.GetWorktree("repo")
	err := wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("testbranch"),
		Create: true,
	})
	assert.NoError(t, err)

	gs.UpdateYaml("repo", "test.yaml", func(o map[string]any) error {
		o["test1"] = map[string]any{
			"test2": 42,
		}
		return nil
	}, "")

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.Master,
	})
	assert.NoError(t, err)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		url, _ := git_url.Parse(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(vc, &types.VarsSource{
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test.yaml",
				Ref:  "testbranch",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_ClusterConfigMap(t *testing.T) {
	cm := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm", Namespace: "ns"},
		Data: map[string]string{
			"vars": `{"test1": {"test2": 42}}`,
		},
	}

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm",
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	}, &cm)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm1",
				Namespace: "ns",
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(t, err, "configmaps \"cm1\" not found")

		err = vl.LoadVars(vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm",
				Namespace: "ns",
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(t, err, "key vars1 not found in ns/ConfigMap/cm on cluster")
	}, &cm)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm-missing",
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)
	}, &cm)
}

func TestVarsLoader_ClusterSecret(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "s", Namespace: "ns"},
		Data: map[string][]byte{
			"vars": []byte(`{"test1": {"test2": 42}}`),
		},
	}

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s",
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	}, &secret)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s1",
				Namespace: "ns",
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(t, err, "secrets \"s1\" not found")

		err = vl.LoadVars(vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s",
				Namespace: "ns",
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(t, err, "key vars1 not found in ns/Secret/s on cluster")
	}, &secret)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s-missing",
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)
	}, &secret)
}

func TestVarsLoader_K8sObjectLabels(t *testing.T) {
	cm1 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm1", Namespace: "ns", Labels: map[string]string{"label1": "value1"}},
		Data: map[string]string{
			"vars": `{"test1": {"test2": 42}}`,
		},
	}
	cm2 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm2", Namespace: "ns", Labels: map[string]string{"label2": "value2"}},
		Data: map[string]string{
			"vars": `{"test3": {"test4": 43}}`,
		},
	}

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label1": "value1"},
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	}, &cm1, &cm2)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label2": "value2"},
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test3", "test4")
		assert.Equal(t, int64(43), v)
	}, &cm1, &cm2)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label2": "value-missing"},
				Namespace: "ns",
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(t, err)
	}, &cm1, &cm2)
}

func TestVarsLoader_SystemEnv(t *testing.T) {
	t.Setenv("TEST1", "42")
	t.Setenv("TEST2", "'43'")
	t.Setenv("TEST4", "44")

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			SystemEnvVars: uo.FromMap(map[string]interface{}{
				"test1": "TEST1",
				"test2": "TEST2",
				"test3": map[string]interface{}{
					"test4": "TEST4",
				},
				"test5": "TEST5:def",
				"test6": "TEST1:def",
				"test7": "TEST5:''",
				"test8": "TEST5:",
			}),
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedField("test1")
		assert.Equal(t, 42., v)

		v, _, _ = vc.Vars.GetNestedField("test2")
		assert.Equal(t, "43", v)

		v, _, _ = vc.Vars.GetNestedField("test3", "test4")
		assert.Equal(t, 44., v)

		v, _, _ = vc.Vars.GetNestedField("test5")
		assert.Equal(t, "def", v)

		v, _, _ = vc.Vars.GetNestedField("test6")
		assert.Equal(t, 42., v)

		v, _, _ = vc.Vars.GetNestedField("test7")
		assert.Equal(t, "", v)

		v, _, _ = vc.Vars.GetNestedField("test8")
		assert.Equal(t, "", v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			SystemEnvVars: uo.FromMap(map[string]interface{}{
				"test5": "TEST5",
			}),
		}, nil, "")
		assert.EqualError(t, err, "environment variable TEST5 not found for test5")
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			SystemEnvVars: uo.FromMap(map[string]interface{}{
				"test1": "TEST-MISSING",
			}),
		}, nil, "")
		assert.NoError(t, err)

		_, ok, _ := vc.Vars.GetNestedField("test1")
		assert.False(t, ok)
	})
}

func TestVarsLoader_Http_GET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ok") {
			_, _ = w.Write([]byte(`{"test1": {"test2": 42}}`))
		} else if strings.HasSuffix(r.URL.Path, "/error") {
			http.Error(w, "error", http.StatusInternalServerError)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		u, _ := url.Parse(ts.URL)
		u.Path += "/ok"

		err := vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		u, _ := url.Parse(ts.URL)
		u.Path += "/missing"
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.NoError(t, err)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		b := true
		u, _ := url.Parse(ts.URL)
		u.Path += "/error"
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed with status code 500")
	})
}

func TestVarsLoader_Http_POST(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(542)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "body" {
			w.WriteHeader(543)
			return
		}
		if r.Header.Get("h") != "h" {
			w.WriteHeader(544)
			return
		}
		_, _ = w.Write([]byte(`{"test1": {"test2": 42}}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.ErrorContains(t, err, "failed with status code 542")

		err = vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:    types.YamlUrl{URL: *u},
				Method: utils.StrPtr("POST"),
			},
		}, nil, "")
		assert.ErrorContains(t, err, "failed with status code 543")

		err = vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:    types.YamlUrl{URL: *u},
				Method: utils.StrPtr("POST"),
				Body:   utils.StrPtr("body"),
			},
		}, nil, "")
		assert.ErrorContains(t, err, "failed with status code 544")

		err = vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:     types.YamlUrl{URL: *u},
				Method:  utils.StrPtr("POST"),
				Body:    utils.StrPtr("body"),
				Headers: map[string]string{"h": "h"},
			},
		}, nil, "")

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_Http_JsonPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"test1": "{\"test2\": 42}"}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		err := vl.LoadVars(vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:      types.YamlUrl{URL: *u},
				JsonPath: utils.StrPtr("test1"),
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test2")
		assert.Equal(t, int64(42), v)
	})
}

func TestVarsLoader_AwsSecretsManager(t *testing.T) {
	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "secret",
			},
		}, nil, "")
		assert.EqualError(t, err, "when omitting the AWS region, the secret name must be a valid ARN")

		err = vl.LoadVars(vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "secret",
				Region:     utils.StrPtr("eu-central1"),
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "arn:aws:secretsmanager:eu-central-1:12345:secret:secret",
			},
		}, nil, "")
		assert.NoError(t, err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(t, int64(42), v)
	})

	testVarsLoader(t, func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		b := true
		err := vl.LoadVars(vc, &types.VarsSource{
			IgnoreMissing: &b,
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "missing",
				Region:     utils.StrPtr("eu-central1"),
			},
		}, nil, "")
		assert.NoError(t, err)
	})
}
