package vars

import (
	"context"
	"fmt"
	"github.com/huandu/xstrings"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/clouds/gcp"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/stretchr/testify/suite"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getsops/sops/v3/age"
	git2 "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kluctl/kluctl/lib/git/auth"
	ssh_pool "github.com/kluctl/kluctl/lib/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VarsLoaderTestSuite struct {
	suite.Suite

	k  *test_utils.EnvTestCluster
	k2 *k8s.K8sCluster
}

func (s *VarsLoaderTestSuite) SetupSuite() {
	s.k = test_utils.CreateEnvTestCluster("cluster")
	err := s.k.Start()
	assert.NoError(s.T(), err)

	discovery, mapper, err := k8s.CreateDiscoveryAndMapper(context.TODO(), s.k.RESTConfig())
	assert.NoError(s.T(), err)
	s.k2, err = k8s.NewK8sCluster(context.TODO(), s.k.RESTConfig(), discovery, mapper, false)
	assert.NoError(s.T(), err)
}

func (s *VarsLoaderTestSuite) TearDownSuite() {
	if s.k != nil {
		s.k.Stop()
	}
}

func (s *VarsLoaderTestSuite) namespace() string {
	n := s.T().Name()
	n = xstrings.ToKebabCase(n)
	n = strings.ReplaceAll(n, "/", "-")
	return n
}

func (s *VarsLoaderTestSuite) createNamespace() {
	err := s.k.Client.Create(context.TODO(), &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: s.namespace()}})
	assert.NoError(s.T(), err)
}

func TestVarsLoader(t *testing.T) {
	suite.Run(t, new(VarsLoaderTestSuite))
}

func (s *VarsLoaderTestSuite) newRP() *repocache.GitRepoCache {
	grc := repocache.NewGitRepoCache(context.TODO(), &ssh_pool.SshPool{}, auth.NewDefaultAuthProviders("KLUCTL_GIT", nil), nil, 0)
	s.T().Cleanup(func() {
		grc.Clear()
	})
	return grc
}

func (s *VarsLoaderTestSuite) testVarsLoader(test func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory)) {
	grc := s.newRP()
	fakeAws := aws.NewFakeClientFactory()
	fakeGcp := gcp.NewFakeClientFactory()

	d := decryptor.NewDecryptor("", decryptor.MaxEncryptedFileSize)
	d.AddLocalKeyService()

	vl := NewVarsLoader(context.TODO(), s.k2, d, grc, fakeAws, fakeGcp)
	vc := NewVarsCtx(newJinja2Must(s.T()))

	test(vl, vc, fakeAws, fakeGcp)
}

func (s *VarsLoaderTestSuite) TestValues() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": {"test2": 42}}`),
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestValuesNoOverrides() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": {"test2": 42}}`),
		}, nil, "")
		assert.NoError(s.T(), err)

		b := true
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values:     uo.FromStringMust(`{"test1": {"test2": 43}}`),
			NoOverride: &b,
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestValuesTargetPath() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values:     uo.FromStringMust(`{"test1": {"test2": 42}}`),
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("my", "target", "test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestWhen() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "a"}`),
		}, nil, "")
		assert.NoError(s.T(), err)
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "b"}`),
			When:   `test1 == "b"`,
		}, nil, "")
		assert.NoError(s.T(), err)
		v, _, _ := vc.Vars.GetNestedString("test1")
		assert.Equal(s.T(), "a", v)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Values: uo.FromStringMust(`{"test1": "b"}`),
			When:   `test1 == "a"`,
		}, nil, "")
		assert.NoError(s.T(), err)
		v, _, _ = vc.Vars.GetNestedString("test1")
		assert.Equal(s.T(), "b", v)
	})
}

func (s *VarsLoaderTestSuite) TestFile() {
	d := s.T().TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": 42}}`), 0o600)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		vs := &types.VarsSource{
			File: utils.Ptr("test.yaml"),
		}
		err := vl.LoadVars(context.TODO(), vc, vs, []string{d}, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
		assert.False(s.T(), vs.RenderedSensitive)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			File:          utils.Ptr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			File:          utils.Ptr("test-missing.yaml"),
		}, []string{d}, "")
		assert.NoError(s.T(), err)
	})
}

func (s *VarsLoaderTestSuite) TestSopsFile() {
	d := s.T().TempDir()
	f, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), f, 0o600)

	s.T().Setenv(age.SopsAgeKeyEnv, string(key))

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		vs := &types.VarsSource{
			File: utils.Ptr("test.yaml"),
		}
		err := vl.LoadVars(context.TODO(), vc, vs, []string{d}, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
		assert.True(s.T(), vs.RenderedSensitive)
	})
}

func (s *VarsLoaderTestSuite) TestFileWithLoad() {
	d := s.T().TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{ load_template("test2.txt") }}}}`), 0o600)
	_ = os.WriteFile(filepath.Join(d, "test2.txt"), []byte(`42`), 0o600)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			File: utils.Ptr("test.yaml"),
		}, []string{d}, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestFileWithLoadSubDir() {
	d := s.T().TempDir()
	_ = os.Mkdir(filepath.Join(d, "subdir"), 0o700)
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{ load_template("test2.txt") }}}}`), 0o600)
	_ = os.WriteFile(filepath.Join(d, "subdir/test2.txt"), []byte(`42`), 0o600)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			File: utils.Ptr("test.yaml"),
		}, []string{d, filepath.Join(d, "subdir")}, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestFileWithLoadNotExists() {
	d := s.T().TempDir()
	_ = os.WriteFile(filepath.Join(d, "test.yaml"), []byte(`{"test1": {"test2": {{load_template("test3.txt")}}}}`), 0o600)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			File: utils.Ptr("test.yaml"),
		}, []string{d}, "")
		assert.EqualError(s.T(), err, "failed to render vars file test.yaml: template test3.txt not found")
	})
}

func (s *VarsLoaderTestSuite) TestGit() {
	gs := test_utils.NewTestGitServer(s.T())
	gs.GitInit("repo")
	gs.UpdateYaml("repo", "test.yaml", func(o map[string]any) error {
		o["test1"] = map[string]any{
			"test2": 42,
		}
		return nil
	}, "")

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := types.ParseGitUrl(gs.GitRepoUrl("repo"))
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test.yaml",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := types.ParseGitUrl(gs.GitRepoUrl("repo"))
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test-missing.yaml",
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})
}

func (s *VarsLoaderTestSuite) TestGitBranch() {
	gs := test_utils.NewTestGitServer(s.T())
	gs.GitInit("repo")

	wt := gs.GetWorktree("repo")
	err := wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("testbranch"),
		Create: true,
	})
	assert.NoError(s.T(), err)

	gs.UpdateYaml("repo", "test.yaml", func(o map[string]any) error {
		o["test1"] = map[string]any{
			"test2": 42,
		}
		return nil
	}, "")

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.Master,
	})
	assert.NoError(s.T(), err)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := types.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Git: &types.VarsSourceGit{
				Url:  *url,
				Path: "test.yaml",
				Ref:  &types.GitRef{Branch: "testbranch"},
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestClusterConfigMap() {
	s.createNamespace()

	cm := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm", Namespace: s.namespace()},
		Data: map[string]string{
			"vars":  `{"test1": {"test2": 42}}`,
			"value": "42",
		},
	}

	err := s.k.Client.Create(context.TODO(), &cm)
	assert.NoError(s.T(), err)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm",
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm1",
				Namespace: s.namespace(),
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, "configmaps \"cm1\" not found")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm",
				Namespace: s.namespace(),
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, fmt.Sprintf("key vars1 not found in %s/ConfigMap/cm on cluster", s.namespace()))
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm-missing",
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "cm",
				Namespace: s.namespace(),
				Key:       "value",
			},
		}, nil, "")
		assert.Errorf(s.T(), err, "failed to load vars from kubernetes object default/ConfigMap/cm and key value: value is not a YAML dictionary")
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Name:       "cm",
				Namespace:  s.namespace(),
				Key:        "value",
				TargetPath: "deep.nested.path",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("deep", "nested", "path")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestClusterSecret() {
	s.createNamespace()

	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "s", Namespace: s.namespace()},
		Data: map[string][]byte{
			"vars": []byte(`{"test1": {"test2": 42}}`),
		},
	}

	err := s.k.Client.Create(context.TODO(), &secret)
	assert.NoError(s.T(), err)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s",
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s1",
				Namespace: s.namespace(),
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, "secrets \"s1\" not found")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s",
				Namespace: s.namespace(),
				Key:       "vars1",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, fmt.Sprintf("key vars1 not found in %s/Secret/s on cluster", s.namespace()))
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		vs := &types.VarsSource{
			IgnoreMissing: &b,
			ClusterSecret: &types.VarsSourceClusterConfigMapOrSecret{
				Name:      "s-missing",
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}
		err := vl.LoadVars(context.TODO(), vc, vs, nil, "")
		assert.NoError(s.T(), err)
		assert.True(s.T(), vs.RenderedSensitive)
	})
}

func (s *VarsLoaderTestSuite) TestK8sObjectLabels() {
	s.createNamespace()

	cm1 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm1", Namespace: s.namespace(), Labels: map[string]string{"label1": "value1"}},
		Data: map[string]string{
			"vars": `{"test1": {"test2": 42}}`,
		},
	}
	cm2 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: "cm2", Namespace: s.namespace(), Labels: map[string]string{"label2": "value2"}},
		Data: map[string]string{
			"vars": `{"test3": {"test4": 43}}`,
		},
	}

	err := s.k.Client.Create(context.TODO(), &cm1)
	assert.NoError(s.T(), err)
	err = s.k.Client.Create(context.TODO(), &cm2)
	assert.NoError(s.T(), err)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label1": "value1"},
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label2": "value2"},
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test3", "test4")
		assert.Equal(s.T(), int64(43), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			ClusterConfigMap: &types.VarsSourceClusterConfigMapOrSecret{
				Labels:    map[string]string{"label2": "value-missing"},
				Namespace: s.namespace(),
				Key:       "vars",
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})
}

func (s *VarsLoaderTestSuite) TestClusterObject() {
	s.createNamespace()

	cm1 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "cm1",
			Namespace: s.namespace(),
			Labels: map[string]string{
				"label1":    "lv1",
				"label2":    "42",
				"label3":    "x",
				"namespace": s.namespace(),
			},
			Annotations: map[string]string{
				"yaml":          `{"x": 45}`,
				"render":        `{{ my.target.a }}`,
				"renderAndYaml": `{"a": {{ my.target.b.x * 2 }} }`,
			},
		},
		Data: map[string]string{},
	}
	cm2 := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "cm2",
			Namespace: s.namespace(),
			Labels: map[string]string{
				"label1":    "lv2",
				"label2":    "44",
				"label3":    "x",
				"namespace": s.namespace(),
			},
			Annotations: map[string]string{
				"a1": "v1",
				"a2": `{{ 1 + 2 }}`,
			},
		},
		Data: map[string]string{},
	}

	err := s.k.Client.Create(context.TODO(), &cm1)
	assert.NoError(s.T(), err)
	err = s.k.Client.Create(context.TODO(), &cm2)
	assert.NoError(s.T(), err)

	// test path
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `$`,
			},
			TargetPath: "my.target2",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels.*`,
			},
			TargetPath: "my.target3",
		}, nil, "")
		assert.ErrorContains(s.T(), err, "json path resulted in multiple matches")

		v, _, _ := vc.Vars.GetNestedString("my", "target")
		assert.Equal(s.T(), "42", v)

		v, _, _ = vc.Vars.GetNestedString("my", "target2", "apiVersion")
		assert.Equal(s.T(), "v1", v)
	})

	// test targetPath
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
		}, nil, "")
		assert.ErrorContains(s.T(), err, `'targetPath' is required for this variable source`)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedString("my", "target")
		assert.Equal(s.T(), "42", v)
	})

	// test targetPath with multiple matches
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		vc.Update(uo.FromMap(map[string]interface{}{
			"list": []map[string]any{
				{
					"x": map[string]any{},
				},
				{
					"x": map[string]any{},
				},
			},
		}))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "list[*].x.y",
		}, nil, "")
		assert.ErrorContains(s.T(), err, `can not deduce what element to add at 'list'`)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "list[0].x.y",
		}, nil, "")
		assert.ErrorContains(s.T(), err, `can not follow a <nil> at 'list[0]'`)
	})

	// test apiVersion
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:       "ConfigMap",
				ApiVersion: "v1",
				Name:       "cm1",
				Namespace:  s.namespace(),
				Path:       `metadata.labels["label2"]`,
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedString("my", "target")
		assert.Equal(s.T(), "42", v)
	})

	// test labels
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind: "ConfigMap",
				Labels: map[string]string{
					"label1": "lv1",
				},
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.a",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind: "ConfigMap",
				Labels: map[string]string{
					"label1": "lv2",
				},
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.b",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind: "ConfigMap",
				Labels: map[string]string{
					"label3": "x",
				},
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.b",
		}, nil, "")
		assert.ErrorContains(s.T(), err, "found more than one object")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind: "ConfigMap",
				Labels: map[string]string{
					"label3": "x",
				},
				List:      true,
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.c",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind: "ConfigMap",
				Labels: map[string]string{
					"namespace": s.namespace(),
				},
				List: true,
				// no namespace
				Path: `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.d",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedString("my", "target", "a")
		assert.Equal(s.T(), "42", v)
		v, _, _ = vc.Vars.GetNestedString("my", "target", "b")
		assert.Equal(s.T(), "44", v)
		l, _, _ := vc.Vars.GetNestedStringList("my", "target", "c")
		assert.Equal(s.T(), []string{"42", "44"}, l)
		l, _, _ = vc.Vars.GetNestedStringList("my", "target", "d")
		assert.Equal(s.T(), []string{"42", "44"}, l)
	})

	// test render
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.a",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.annotations["render"]`,
			},
			TargetPath: "my.target.b",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.annotations["render"]`,
				Render:    true,
			},
			TargetPath: "my.target.c",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm2",
				Namespace: s.namespace(),
				Path:      `metadata.annotations`,
				Render:    true,
			},
			TargetPath: "my.target.d",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedString("my", "target", "a")
		assert.Equal(s.T(), "42", v)
		v, _, _ = vc.Vars.GetNestedString("my", "target", "b")
		assert.Equal(s.T(), `{{ my.target.a }}`, v)
		v, _, _ = vc.Vars.GetNestedString("my", "target", "c")
		assert.Equal(s.T(), "42", v)
		x, _, _ := vc.Vars.GetNestedField("my", "target", "d")
		assert.Equal(s.T(), map[string]any{
			"a1": "v1",
			"a2": "3",
		}, x)
	})

	// test parseYaml
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.labels["label2"]`,
			},
			TargetPath: "my.target.a",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.annotations["yaml"]`,
				ParseYaml: true,
			},
			TargetPath: "my.target.b",
		}, nil, "")
		assert.NoError(s.T(), err)

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			ClusterObject: &types.VarsSourceClusterObject{
				Kind:      "ConfigMap",
				Name:      "cm1",
				Namespace: s.namespace(),
				Path:      `metadata.annotations["renderAndYaml"]`,
				Render:    true,
				ParseYaml: true,
			},
			TargetPath: "my.target.c",
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedString("my", "target", "a")
		assert.Equal(s.T(), "42", v)
		x, _, _ := vc.Vars.GetNestedField("my", "target", "b")
		assert.Equal(s.T(), uo.FromMap(map[string]any{
			"x": float64(45),
		}), x)
		x, _, _ = vc.Vars.GetNestedField("my", "target", "c")
		assert.Equal(s.T(), uo.FromMap(map[string]any{
			"a": float64(90),
		}), x)
	})
}

func (s *VarsLoaderTestSuite) TestSystemEnv() {
	s.T().Setenv("TEST1", "42")
	s.T().Setenv("TEST2", "'43'")
	s.T().Setenv("TEST4", "44")

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
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
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedField("test1")
		assert.Equal(s.T(), 42., v)

		v, _, _ = vc.Vars.GetNestedField("test2")
		assert.Equal(s.T(), "43", v)

		v, _, _ = vc.Vars.GetNestedField("test3", "test4")
		assert.Equal(s.T(), 44., v)

		v, _, _ = vc.Vars.GetNestedField("test5")
		assert.Equal(s.T(), "def", v)

		v, _, _ = vc.Vars.GetNestedField("test6")
		assert.Equal(s.T(), 42., v)

		v, _, _ = vc.Vars.GetNestedField("test7")
		assert.Equal(s.T(), "", v)

		v, _, _ = vc.Vars.GetNestedField("test8")
		assert.Equal(s.T(), "", v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			SystemEnvVars: uo.FromMap(map[string]interface{}{
				"test5": "TEST5",
			}),
		}, nil, "")
		assert.EqualError(s.T(), err, "environment variable TEST5 not found for test5")
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			SystemEnvVars: uo.FromMap(map[string]interface{}{
				"test1": "TEST-MISSING",
			}),
		}, nil, "")
		assert.NoError(s.T(), err)

		_, ok, _ := vc.Vars.GetNestedField("test1")
		assert.False(s.T(), ok)
	})
}

func (s *VarsLoaderTestSuite) TestHttp_GET() {
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

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		u, _ := url.Parse(ts.URL)
		u.Path += "/ok"

		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		u, _ := url.Parse(ts.URL)
		u.Path += "/missing"
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		b := true
		u, _ := url.Parse(ts.URL)
		u.Path += "/error"
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.Error(s.T(), err)
		assert.Contains(s.T(), err.Error(), "failed with status code 500")
	})
}

func (s *VarsLoaderTestSuite) TestHttp_POST() {
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

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url: types.YamlUrl{URL: *u},
			},
		}, nil, "")
		assert.ErrorContains(s.T(), err, "failed with status code 542")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:    types.YamlUrl{URL: *u},
				Method: utils.Ptr("POST"),
			},
		}, nil, "")
		assert.ErrorContains(s.T(), err, "failed with status code 543")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:    types.YamlUrl{URL: *u},
				Method: utils.Ptr("POST"),
				Body:   utils.Ptr("body"),
			},
		}, nil, "")
		assert.ErrorContains(s.T(), err, "failed with status code 544")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:     types.YamlUrl{URL: *u},
				Method:  utils.Ptr("POST"),
				Body:    utils.Ptr("body"),
				Headers: map[string]string{"h": "h"},
			},
		}, nil, "")

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestHttp_JsonPath() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"test1": "{\"test2\": 42}"}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			Http: &types.VarsSourceHttp{
				Url:      types.YamlUrl{URL: *u},
				JsonPath: utils.Ptr("test1"),
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test2")
		assert.Equal(s.T(), int64(42), v)
	})
}

func (s *VarsLoaderTestSuite) TestAwsSecretsManager() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "secret",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, "when omitting the AWS region, the secret name must be a valid ARN")

		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "secret",
				Region:     utils.Ptr("eu-central1"),
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "arn:aws:secretsmanager:eu-central-1:12345:secret:secret",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		aws.Secrets = map[string]string{
			"secret": `{"test1": {"test2": 42}}`,
		}

		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			AwsSecretsManager: &types.VarsSourceAwsSecretsManager{
				SecretName: "missing",
				Region:     utils.Ptr("eu-central1"),
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})
}

func (s *VarsLoaderTestSuite) TestGcpSecretManager() {
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		gcp.Secrets = map[string]string{
			"projects/my-project/secrets/secret/versions/latest": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GcpSecretManager: &types.VarsSourceGcpSecretManager{
				SecretName: "projects/my-project/secrets/secret/versions/latest",
			},
		}, nil, "")
		assert.NoError(s.T(), err)

		v, _, _ := vc.Vars.GetNestedInt("test1", "test2")
		assert.Equal(s.T(), int64(42), v)
	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		gcp.Secrets = map[string]string{
			"projects/my-project/secrets/secret/versions/latest": `{"test1": {"test2": 42}}`,
		}

		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GcpSecretManager: &types.VarsSourceGcpSecretManager{
				SecretName: "projects/different-project/secrets/secret/versions/latest",
			},
		}, nil, "")
		assert.EqualError(s.T(), err, "secret not found: failed to access secret version: rpc error: code = NotFound desc = secret not found: failed to access secret version: rpc error: code = NotFound desc = secret projects/different-project/secrets/secret/versions/latest not found")

	})

	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		gcp.Secrets = map[string]string{
			"projects/my-project/secrets/secret/versions/latest": `{"test1": {"test2": 42}}`,
		}

		b := true
		err := vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			IgnoreMissing: &b,
			GcpSecretManager: &types.VarsSourceGcpSecretManager{
				SecretName: "projects/my-project/secrets/missing-secret/versions/latest",
			},
		}, nil, "")
		assert.NoError(s.T(), err)
	})
}
