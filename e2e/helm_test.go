package e2e

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type helmTestCase struct {
	path              string
	name              string
	helmType          test_utils.TestHelmRepoType
	testAuth          bool
	testTLS           bool
	testTLSClientCert bool
	credsId           string

	argCredsId   string
	argCredsHost string
	argCredsPath string
	argUsername  string
	argPassword  string

	argPassCA         bool
	argPassClientCert bool

	expectedReadyError   string
	expectedPrepareError string
}

var helmTests = []helmTestCase{
	{name: "helm-no-creds"},
	{name: "oci-no-creds", helmType: test_utils.TestHelmRepo_Oci},
	{name: "git-no-creds", helmType: test_utils.TestHelmRepo_Git},

	// tls tests
	{
		name: "helm-tls-missing-ca", testTLS: true,
		argPassCA:            false,
		argCredsHost:         "<host>",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "failed to verify certificate",
	},
	{
		name: "helm-tls-valid-ca", testTLS: true,
		argPassCA:    true,
		argCredsHost: "<host>",
	},
	{
		name: "helm-tls-missing-cert", testTLS: true, testTLSClientCert: true,
		argPassCA:            true,
		argPassClientCert:    false,
		argCredsHost:         "<host>",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "certificate required",
	},
	{
		name: "helm-tls-valid-cert", testTLS: true, testTLSClientCert: true,
		argPassCA:         true,
		argPassClientCert: true,
		argCredsHost:      "<host>",
	},

	{
		name: "oci-tls-missing-ca", helmType: test_utils.TestHelmRepo_Oci, testTLS: true,
		argPassCA:            false,
		argCredsHost:         "<host>",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "failed to verify certificate",
	},
	{
		name: "oci-tls-valid-ca", helmType: test_utils.TestHelmRepo_Oci, testTLS: true,
		argPassCA:    true,
		argCredsHost: "<host>",
	},
	{
		name: "oci-tls-missing-cert", helmType: test_utils.TestHelmRepo_Oci, testTLS: true, testTLSClientCert: true,
		argPassCA:            true,
		argPassClientCert:    false,
		argCredsHost:         "<host>",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "certificate required",
	},
	{
		name: "oci-tls-valid-cert", helmType: test_utils.TestHelmRepo_Oci, testTLS: true, testTLSClientCert: true,
		argPassCA:         true,
		argPassClientCert: true,
		argCredsHost:      "<host>",
	},

	// deprecated helm credentials flags
	{
		name: "dep-helm-creds-missing", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, credsId: "test-creds",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "dep-helm-creds-invalid", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, credsId: "test-creds",
		argCredsId: "test-creds", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "dep-helm-creds-valid", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, credsId: "test-creds",
		argCredsId: "test-creds", argUsername: "test-user", argPassword: "secret-password",
	},
	{
		name: "dep-oci-creds-fail", helmType: test_utils.TestHelmRepo_Oci, testAuth: true, credsId: "test-creds",
		argCredsId: "test-creds", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "OCI charts can currently only be authenticated via registry login and environment variables but not via cli arguments",
	},

	// new helm credentials flags
	{
		name: "helm-creds-missing", helmType: test_utils.TestHelmRepo_Helm, testAuth: true,
		argCredsHost: "<host>-invalid", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "helm-creds-invalid", helmType: test_utils.TestHelmRepo_Helm, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "helm-creds-valid", helmType: test_utils.TestHelmRepo_Helm, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "secret-password",
	},
	{
		name: "helm-creds-missing-path", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, path: "path1",
		argCredsHost: "<host>", argCredsPath: "path2", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "helm-creds-invalid-path", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, path: "path1",
		argCredsHost: "<host>", argCredsPath: "path1", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "helm-creds-valid-path", helmType: test_utils.TestHelmRepo_Helm, testAuth: true, path: "path1",
		argCredsHost: "<host>", argCredsPath: "path1", argUsername: "test-user", argPassword: "secret-password",
	},

	// oci creds
	{
		name: "oci-creds-missing", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>-invalid", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "no basic auth credentials",
	},
	{
		name: "oci-creds-invalid", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "oci-creds-valid", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "secret-password",
	},
	{
		name: "oci-creds-missing-path", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "test-chart2", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "no basic auth credentials",
	},
	{
		name: "oci-creds-invalid-path", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "test-chart1", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "401 Unauthorized",
	},
	{
		name: "oci-creds-valid-path", helmType: test_utils.TestHelmRepo_Oci, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "test-chart1", argUsername: "test-user", argPassword: "secret-password",
	},

	// git creds
	{
		name: "git-creds-missing", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>-invalid", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "authentication required",
	},
	{
		name: "git-creds-invalid", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "authentication required",
	},
	{
		name: "git-creds-valid", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>", argUsername: "test-user", argPassword: "secret-password",
	},
	{
		name: "git-creds-missing-path", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "*/helm-repo2", argUsername: "test-user", argPassword: "secret-password",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "authentication required",
	},
	{
		name: "git-creds-invalid-path", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "*/helm-repo", argUsername: "test-user", argPassword: "invalid",
		expectedReadyError:   "prepare failed with 1 errors. Check status.lastPrepareError for details",
		expectedPrepareError: "authentication required",
	},
	{
		name: "git-creds-valid-path", helmType: test_utils.TestHelmRepo_Git, testAuth: true,
		argCredsHost: "<host>", argCredsPath: "*/helm-repo", argUsername: "test-user", argPassword: "secret-password",
	},
}

func newTmpFile(t *testing.T, b []byte) string {
	tmp, _ := os.Create(filepath.Join(t.TempDir(), "x.pem"))
	defer tmp.Close()
	tmp.Write(b)
	return tmp.Name()
}

func buildHelmTestExtraArgs(t *testing.T, tc helmTestCase, repo *test_utils.TestHelmRepo) []string {
	var ret []string
	if tc.helmType == test_utils.TestHelmRepo_Oci {
		if tc.argCredsHost != "" {
			r := strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host)
			if tc.argCredsPath != "" {
				r += "/" + tc.argCredsPath
			}
			if (rand.Int() & 1) == 0 {
				ret = append(ret, fmt.Sprintf("--registry-username=%s=%s", r, tc.argUsername))
				ret = append(ret, fmt.Sprintf("--registry-password=%s=%s", r, tc.argPassword))
			} else {
				ret = append(ret, fmt.Sprintf("--registry-creds=%s=%s:%s", r, tc.argUsername, tc.argPassword))
			}
			if !repo.HttpServer.TLSEnabled {
				ret = append(ret, fmt.Sprintf("--registry-plain-http=%s", r))
			}
		}
		if !repo.HttpServer.TLSEnabled {
			r := repo.URL.Host
			if tc.argCredsPath != "" {
				r += "/" + tc.argCredsPath
			}
			ret = append(ret, fmt.Sprintf("--registry-plain-http=%s", r))
		}
		if tc.testTLS {
			if tc.argPassCA {
				ret = append(ret, fmt.Sprintf("--registry-ca-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ServerCAs)))
			}
			if tc.argPassClientCert {
				ret = append(ret, fmt.Sprintf("--registry-cert-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ClientCert)))
				ret = append(ret, fmt.Sprintf("--registry-key-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ClientKey)))
			}
		}
	} else if tc.helmType == test_utils.TestHelmRepo_Helm {
		if tc.argCredsId != "" {
			ret = append(ret, fmt.Sprintf("--helm-username=%s:%s", tc.argCredsId, tc.argUsername))
			ret = append(ret, fmt.Sprintf("--helm-password=%s:%s", tc.argCredsId, tc.argPassword))
		} else if tc.argCredsHost != "" {
			r := strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host)
			if tc.argCredsPath != "" {
				r += "/" + tc.argCredsPath
			}
			if (rand.Int() & 1) == 0 {
				ret = append(ret, fmt.Sprintf("--helm-username=%s=%s", r, tc.argUsername))
				ret = append(ret, fmt.Sprintf("--helm-password=%s=%s", r, tc.argPassword))
			} else {
				ret = append(ret, fmt.Sprintf("--helm-creds=%s=%s:%s", r, tc.argUsername, tc.argPassword))
			}
		}
		if tc.testTLS {
			if tc.argPassCA {
				ret = append(ret, fmt.Sprintf("--helm-ca-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ServerCAs)))
			}
			if tc.argPassClientCert {
				ret = append(ret, fmt.Sprintf("--helm-cert-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ClientCert)))
				ret = append(ret, fmt.Sprintf("--helm-key-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ClientKey)))
			}
		}
	} else if tc.helmType == test_utils.TestHelmRepo_Git {
		if tc.argCredsHost != "" {
			r := strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host)
			if tc.argCredsPath != "" {
				r += "/" + tc.argCredsPath
			}
			ret = append(ret, fmt.Sprintf("--git-username=%s=%s", r, tc.argUsername))
			ret = append(ret, fmt.Sprintf("--git-password=%s=%s", r, tc.argPassword))
		}
		if tc.argPassCA {
			ret = append(ret, fmt.Sprintf("--git-ca-file=%s=%s", repo.URL.Host, newTmpFile(t, repo.HttpServer.ServerCAs)))
		}
	}
	// add a fallback that enables plain_http in case we have no matching creds
	if tc.helmType == test_utils.TestHelmRepo_Oci && !repo.HttpServer.TLSEnabled {
		ret = append(ret, fmt.Sprintf("--registry-plain-http=%s", repo.URL.Host))
	}
	return ret
}

func buildHelmTestEnvVars(t *testing.T, tc helmTestCase, p *test_project.TestProject, repo *test_utils.TestHelmRepo) {
	setEnv := func(k string, v string) {
		if p.IsUseProcess() {
			p.SetEnv(k, v)
		} else {
			t.Setenv(k, v)
		}
	}

	if tc.helmType == test_utils.TestHelmRepo_Oci {
		if tc.argCredsHost != "" {
			setEnv("KLUCTL_REGISTRY_HOST", strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host))
		}
		if tc.argCredsPath != "" {
			setEnv("KLUCTL_REGISTRY_REPOSITORY", tc.argCredsPath)
		}
		if tc.argUsername != "" {
			setEnv("KLUCTL_REGISTRY_USERNAME", tc.argUsername)
		}
		if tc.argPassword != "" {
			setEnv("KLUCTL_REGISTRY_PASSWORD", tc.argPassword)
		}
		if !repo.HttpServer.TLSEnabled {
			setEnv("KLUCTL_REGISTRY_PLAIN_HTTP", "true")
		}
		if tc.argPassCA {
			setEnv("KLUCTL_REGISTRY_CA_FILE", newTmpFile(t, repo.HttpServer.ServerCAs))
		}
		if tc.argPassClientCert {
			setEnv("KLUCTL_REGISTRY_CERT_FILE", newTmpFile(t, repo.HttpServer.ClientCert))
			setEnv("KLUCTL_REGISTRY_KEY_FILE", newTmpFile(t, repo.HttpServer.ClientKey))
		}
	} else if tc.helmType == test_utils.TestHelmRepo_Helm {
		if tc.argCredsId != "" {
			setEnv("KLUCTL_HELM_CREDENTIALS_ID", tc.argCredsId)
		}
		if tc.argCredsHost != "" {
			setEnv("KLUCTL_HELM_HOST", strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host))
		}
		if tc.argCredsPath != "" {
			setEnv("KLUCTL_HELM_PATH", tc.argCredsPath)
		}
		if tc.argUsername != "" {
			setEnv("KLUCTL_HELM_USERNAME", tc.argUsername)
		}
		if tc.argPassword != "" {
			setEnv("KLUCTL_HELM_PASSWORD", tc.argPassword)
		}
		if tc.argPassCA {
			setEnv("KLUCTL_HELM_CA_FILE", newTmpFile(t, repo.HttpServer.ServerCAs))
		}
		if tc.argPassClientCert {
			setEnv("KLUCTL_HELM_CERT_FILE", newTmpFile(t, repo.HttpServer.ClientCert))
			setEnv("KLUCTL_HELM_KEY_FILE", newTmpFile(t, repo.HttpServer.ClientKey))
		}
	} else if tc.helmType == test_utils.TestHelmRepo_Git {
		if tc.argCredsHost != "" {
			setEnv("KLUCTL_GIT_HOST", strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host))
		}
		if tc.argCredsPath != "" {
			setEnv("KLUCTL_GIT_PATH", tc.argCredsPath)
		}
		if tc.argUsername != "" {
			setEnv("KLUCTL_GIT_USERNAME", tc.argUsername)
		}
		if tc.argPassword != "" {
			setEnv("KLUCTL_GIT_PASSWORD", tc.argPassword)
		}
	}
	// add a fallback that enables plain_http in case we have no matching creds
	if tc.helmType == test_utils.TestHelmRepo_Oci && !repo.HttpServer.TLSEnabled {
		setEnv("KLUCTL_REGISTRY_1_HOST", repo.URL.Host)
		setEnv("KLUCTL_REGISTRY_1_PLAIN_HTTP", "true")
	}
}

func prepareHelmTestCase(t *testing.T, k *test_utils.EnvTestCluster, tc helmTestCase, prePull bool, useProcess bool, libraryMode libraryTestMode) (*test_project.TestProject, *test_utils.TestHelmRepo, error) {
	gitServer := test_utils.NewTestGitServer(t)
	gitSubDir := ""

	if libraryMode == includeLibrary {
		gitSubDir = "include"
	}

	p := test_project.NewTestProject(t,
		test_project.WithUseProcess(useProcess),
		test_project.WithBareProject(),
		test_project.WithGitServer(gitServer),
		test_project.WithGitSubDir(gitSubDir),
	)

	if libraryMode != noLibrary {
		p.UpdateYaml(".kluctl-library.yaml", func(o *uo.UnstructuredObject) error {
			return nil
		}, "")
	} else {
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			return nil
		})
	}

	createNamespace(t, k, p.TestSlug())

	user := ""
	password := ""
	if tc.testAuth {
		user = "test-user"
		password = "secret-password"
	}

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}

	var repo *test_utils.TestHelmRepo
	if tc.helmType == test_utils.TestHelmRepo_Helm || tc.helmType == test_utils.TestHelmRepo_Oci {
		repo = test_utils.NewHelmTestRepo(tc.helmType, tc.path, charts)
		repo.HttpServer.Username = user
		repo.HttpServer.Password = password
		repo.HttpServer.TLSEnabled = tc.testTLS
		repo.HttpServer.TLSClientCertEnabled = tc.testTLSClientCert
		repo.HttpServer.NoLoopbackProxyEnabled = true
	} else if tc.helmType == test_utils.TestHelmRepo_Git {
		repo = test_utils.NewHelmTestRepoGit(t, tc.path, charts, user, password)
	}

	repo.Start(t)

	extraArgs := buildHelmTestExtraArgs(t, tc, repo)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	if tc.testAuth {
		if tc.credsId != "" {
			p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
				_ = o.SetNestedField(tc.credsId, "helmChart", "credentialsId")
				return nil
			}, "")
		}
	}

	if prePull {
		args := []string{"helm-pull"}
		args = append(args, extraArgs...)

		_, stderr, err := p.Kluctl(t, args...)
		if tc.expectedPrepareError != "" {
			assert.Error(t, err)
			assert.Contains(t, stderr, tc.expectedPrepareError)
			return p, repo, err
		} else {
			assert.NoError(t, err)
			assert.FileExists(t, getChartFile(t, p, repo, "test-chart1", "0.1.0"))

			p.GitServer().CommitFiles(p.GitRepoName(), []string{filepath.Join(gitSubDir, ".helm-charts")}, true, "helm-pull")
		}
	} else {
		p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(true, "helmChart", "skipPrePull")
			return nil
		}, "")
	}

	if libraryMode == gitLibrary {
		p2 := test_project.NewTestProject(t,
			test_project.WithUseProcess(useProcess),
			test_project.WithBareProject(),
		)
		p2.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"git": map[string]any{
				"url": p.GitUrl(),
			},
		}))
		return p2, repo, nil
	} else if libraryMode == includeLibrary {
		p2 := test_project.NewTestProject(t,
			test_project.WithUseProcess(useProcess),
			test_project.WithBareProject(),
			test_project.WithGitServer(gitServer),
			test_project.WithGitSubDir("project"),
		)
		p2.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"include": "../include",
		}))
		return p2, repo, nil
	}

	return p, repo, nil
}

type libraryTestMode int

const (
	noLibrary = iota
	gitLibrary
	includeLibrary
)

func testHelmPull(t *testing.T, tc helmTestCase, prePull bool, credsViaEnv bool, libraryMode libraryTestMode) {
	useProcess := credsViaEnv

	// uncomment this if you want to debug this when credsViaEnv==true
	// useProcess = false

	if !credsViaEnv || useProcess {
		t.Parallel()
	}

	k := defaultCluster1
	p, repo, err := prepareHelmTestCase(t, k, tc, prePull, useProcess, libraryMode)
	if err != nil {
		if tc.expectedPrepareError == "" {
			assert.Fail(t, "did not expect error")
		}
		return
	}

	args := []string{"deploy", "--yes"}
	if credsViaEnv {
		buildHelmTestEnvVars(t, tc, p, repo)
	} else {
		args = append(args, buildHelmTestExtraArgs(t, tc, repo)...)
	}

	_, stderr, err := p.Kluctl(t, args...)
	pullMessage := "Pulling Helm Chart test-chart1 with version 0.1.0"
	if tc.helmType == test_utils.TestHelmRepo_Git {
		pullMessage = "Pulling Helm Chart test-chart1 with version refs/tags/0.1.0"
	}
	if prePull {
		assert.NotContains(t, stderr, pullMessage)
	} else {
		assert.Contains(t, stderr, pullMessage)
	}
	if tc.expectedPrepareError != "" {
		if useProcess {
			assert.Contains(t, stderr, tc.expectedPrepareError)
		} else {
			assert.ErrorContains(t, err, tc.expectedPrepareError)
		}
	} else {
		assert.NoError(t, err)
		assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	}
}

func TestHelmPull(t *testing.T) {
	for _, tc := range helmTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, false, false, noLibrary)
		})
	}
}

func TestHelmPrePull(t *testing.T) {
	for _, tc := range helmTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, true, false, noLibrary)
		})
	}
}

func TestHelmPullCredsViaEnv(t *testing.T) {
	for _, tc := range helmTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, false, true, noLibrary)
		})
	}
}

func TestHelmInGitLibrary(t *testing.T) {
	for _, tc := range helmTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, true, false, gitLibrary)
		})
	}
}

func TestHelmInIncludeLibrary(t *testing.T) {
	for _, tc := range helmTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, true, false, includeLibrary)
		})
	}
}

func testHelmManualUpgrade(t *testing.T, helmType test_utils.TestHelmRepoType) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}

	var repo *test_utils.TestHelmRepo
	switch helmType {
	case test_utils.TestHelmRepo_Helm, test_utils.TestHelmRepo_Oci:
		repo = test_utils.NewHelmTestRepo(helmType, "", charts)
	case test_utils.TestHelmRepo_Git:
		repo = test_utils.NewHelmTestRepoGit(t, "", charts, "", "")
	}
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	p.KluctlMust(t, "helm-pull")
	assert.FileExists(t, getChartFile(t, p, repo, "test-chart1", "0.1.0"))
	p.KluctlMust(t, "deploy", "--yes")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	v, _, _ := cm.GetNestedString("data", "version")
	assert.Equal(t, "0.1.0", v)

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		if helmType == test_utils.TestHelmRepo_Git {
			_ = o.SetNestedField("0.2.0", "helmChart", "git", "ref", "tag")
		} else {
			_ = o.SetNestedField("0.2.0", "helmChart", "chartVersion")
		}
		return nil
	}, "")

	p.KluctlMust(t, "helm-pull")
	assert.NoFileExists(t, getChartFile(t, p, repo, "test-chart1", "0.1.0"))
	assert.FileExists(t, getChartFile(t, p, repo, "test-chart1", "0.2.0"))
	p.KluctlMust(t, "deploy", "--yes")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	v, _, _ = cm.GetNestedString("data", "version")
	assert.Equal(t, "0.2.0", v)
}

func TestHelmManualUpgrade(t *testing.T) {
	testHelmManualUpgrade(t, test_utils.TestHelmRepo_Helm)
}

func TestHelmManualUpgradeOci(t *testing.T) {
	testHelmManualUpgrade(t, test_utils.TestHelmRepo_Oci)
}

func TestHelmManualUpgradeGit(t *testing.T) {
	testHelmManualUpgrade(t, test_utils.TestHelmRepo_Git)
}

func testHelmUpdate(t *testing.T, helmType test_utils.TestHelmRepoType, upgrade bool, commit bool) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts1 := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}
	charts2 := []test_utils.RepoChart{
		{ChartName: "test-chart2", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.3.0"},
	}
	var repo1, repo2 *test_utils.TestHelmRepo
	switch helmType {
	case test_utils.TestHelmRepo_Helm, test_utils.TestHelmRepo_Oci:
		repo1 = test_utils.NewHelmTestRepo(helmType, "", charts1)
		repo2 = test_utils.NewHelmTestRepo(helmType, "", charts2)
	case test_utils.TestHelmRepo_Git:
		repo1 = test_utils.NewHelmTestRepoGit(t, "", charts1, "", "")
		repo2 = test_utils.NewHelmTestRepoGit(t, "", charts2, "", "")
	}
	repo1.Start(t)
	repo2.Start(t)

	p.AddHelmDeployment("helm1", repo1, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repo2, "test-chart2", "0.1.0", "test-helm2", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repo1, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), nil)

	p.UpdateYaml("helm3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipUpdate")
		return nil
	}, "")

	p.KluctlMust(t, "helm-pull")
	assert.FileExists(t, getChartFile(t, p, repo1, "test-chart1", "0.1.0"))
	assert.FileExists(t, getChartFile(t, p, repo2, "test-chart2", "0.1.0"))

	if commit {
		wt, _ := p.GetGitRepo().Worktree()
		_, _ = wt.Add(".helm-charts")
		_, _ = wt.Commit(".helm-charts", &git.CommitOptions{})
	}

	args := []string{"helm-update"}
	if upgrade {
		args = append(args, "--upgrade")
	}
	if commit {
		args = append(args, "--commit")
	}

	versionPrefix := ""
	if helmType == test_utils.TestHelmRepo_Git {
		versionPrefix = "refs/tags/"
	}

	_, stderr := p.KluctlMust(t, args...)
	assert.Contains(t, stderr, fmt.Sprintf("helm1: Chart test-chart1 (old version %s0.1.0) has new version %s0.2.0 available", versionPrefix, versionPrefix))
	assert.Contains(t, stderr, fmt.Sprintf("helm2: Chart test-chart2 (old version %s0.1.0) has new version %s0.3.0 available", versionPrefix, versionPrefix))
	assert.Contains(t, stderr, fmt.Sprintf("helm3: Skipped update to version %s0.2.0", versionPrefix))

	if upgrade {
		assert.Contains(t, stderr, fmt.Sprintf("Upgrading Chart test-chart1 from version %s0.1.0 to %s0.2.0", versionPrefix, versionPrefix))
		assert.Contains(t, stderr, fmt.Sprintf("Upgrading Chart test-chart2 from version %s0.1.0 to %s0.3.0", versionPrefix, versionPrefix))
	}
	if commit {
		assert.Contains(t, stderr, fmt.Sprintf("Committed helm chart test-chart1 with version %s0.2.0", versionPrefix))
		assert.Contains(t, stderr, fmt.Sprintf("Committed helm chart test-chart2 with version %s0.3.0", versionPrefix))
	}

	pulledVersions1 := listChartVersions(t, p, repo1, "test-chart1")
	pulledVersions2 := listChartVersions(t, p, repo2, "test-chart2")

	if upgrade {
		assert.Equal(t, []string{"0.1.0", "0.2.0"}, pulledVersions1)
		assert.Equal(t, []string{"0.3.0"}, pulledVersions2)
	} else {
		assert.Equal(t, []string{"0.1.0"}, pulledVersions1)
		assert.Equal(t, []string{"0.1.0"}, pulledVersions2)
	}

	if commit {
		r := p.GetGitRepo()

		commits, err := r.Log(&git.LogOptions{})
		assert.NoError(t, err)
		var commitList []object.Commit
		err = commits.ForEach(func(commit *object.Commit) error {
			commitList = append(commitList, *commit)
			return nil
		})
		assert.NoError(t, err)

		commitList = commitList[0:2]
		sort.Slice(commitList, func(i, j int) bool {
			return commitList[i].Message < commitList[j].Message
		})

		assert.Equal(t, fmt.Sprintf("Updated helm chart test-chart1 from version %s0.1.0 to version %s0.2.0", versionPrefix, versionPrefix), commitList[0].Message)
		assert.Equal(t, fmt.Sprintf("Updated helm chart test-chart2 from version %s0.1.0 to version %s0.3.0", versionPrefix, versionPrefix), commitList[1].Message)
	}
}

func TestHelmUpdate(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Helm, false, false)
}

func TestHelmUpdateOci(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Oci, false, false)
}

func TestHelmUpdateGit(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Git, false, false)
}

func TestHelmUpdateAndUpgrade(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Helm, true, false)
}

func TestHelmUpdateAndUpgradeOci(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Oci, true, false)
}

func TestHelmUpdateAndUpgradeGit(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Git, true, false)
}

func TestHelmUpdateAndUpgradeAndCommit(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Helm, true, true)
}

func TestHelmUpdateAndUpgradeAndCommitOci(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Oci, true, true)
}

func TestHelmUpdateAndUpgradeAndCommitGit(t *testing.T) {
	testHelmUpdate(t, test_utils.TestHelmRepo_Git, true, true)
}

func testHelmUpdateConstraints(t *testing.T, helmType test_utils.TestHelmRepoType) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.1.1"},
		{ChartName: "test-chart1", Version: "0.2.0"},
		{ChartName: "test-chart1", Version: "1.1.0"},
		{ChartName: "test-chart1", Version: "1.1.1"},
		{ChartName: "test-chart1", Version: "1.2.1"},
	}
	repo := test_utils.NewHelmTestRepo(helmType, "", charts)
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repo, "test-chart1", "0.1.0", "test-helm2", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repo, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), nil)

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~0.1.0", "helmChart", "updateConstraints")
		return nil
	}, "")
	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~0.2.0", "helmChart", "updateConstraints")
		return nil
	}, "")
	p.UpdateYaml("helm3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~1.2.0", "helmChart", "updateConstraints")
		return nil
	}, "")

	args := []string{"helm-update", "--upgrade"}

	_, stderr := p.KluctlMust(t, args...)
	assert.Contains(t, stderr, "helm1: Chart test-chart1 (old version 0.1.0) has new version 0.1.1 available")
	assert.Contains(t, stderr, "helm2: Chart test-chart1 (old version 0.1.0) has new version 0.2.0 available")
	assert.Contains(t, stderr, "helm3: Chart test-chart1 (old version 0.1.0) has new version 1.2.1 available")

	c1 := p.GetYaml("helm1/helm-chart.yaml")
	c2 := p.GetYaml("helm2/helm-chart.yaml")
	c3 := p.GetYaml("helm3/helm-chart.yaml")

	v1, _, _ := c1.GetNestedString("helmChart", "chartVersion")
	v2, _, _ := c2.GetNestedString("helmChart", "chartVersion")
	v3, _, _ := c3.GetNestedString("helmChart", "chartVersion")
	assert.Equal(t, "0.1.1", v1)
	assert.Equal(t, "0.2.0", v2)
	assert.Equal(t, "1.2.1", v3)
}

func TestHelmUpdateConstraints(t *testing.T) {
	testHelmUpdateConstraints(t, test_utils.TestHelmRepo_Helm)
}

func TestHelmUpdateConstraintsOci(t *testing.T) {
	testHelmUpdateConstraints(t, test_utils.TestHelmRepo_Oci)
}

func TestHelmValues(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.1.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	values1 := map[string]any{
		"data": map[string]any{
			"a": "x1",
			"b": "y1",
		},
	}
	values2 := map[string]any{
		"data": map[string]any{
			"a": "x2",
			"b": "y2",
		},
	}
	values3 := map[string]any{
		"data": map[string]any{
			"a": "{{ args.a }}",
			"b": "{{ args.b }}",
		},
	}

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), values1)
	p.AddHelmDeployment("helm2", repo, "test-chart2", "0.1.0", "test-helm2", p.TestSlug(), values2)
	p.AddHelmDeployment("helm3", repo, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), values3)

	p.KluctlMust(t, "helm-pull")
	p.KluctlMust(t, "deploy", "--yes", "-aa=a", "-ab=b")

	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	cm2 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm2-test-chart2")
	cm3 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm3-test-chart1")

	assert.Equal(t, map[string]any{
		"a":           "x1",
		"b":           "y1",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm1.Object["data"])
	assert.Equal(t, map[string]any{
		"a":           "x2",
		"b":           "y2",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm2.Object["data"])
	assert.Equal(t, map[string]any{
		"a":           "a",
		"b":           "b",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm3.Object["data"])
}

func TestHelmTemplateChartYaml(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())
	createNamespace(t, k, p.TestSlug()+"-a")
	createNamespace(t, k, p.TestSlug()+"-b")

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.1.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm-{{ args.a }}", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repo, "test-chart2", "0.1.0", "test-helm-{{ args.b }}", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repo, "test-chart1", "0.1.0", "test-helm-ns", p.TestSlug()+"-{{ args.a }}", nil)
	p.AddHelmDeployment("helm4", repo, "test-chart1", "0.1.0", "test-helm-ns", p.TestSlug()+"-{{ args.b }}", nil)

	p.KluctlMust(t, "helm-pull")
	p.KluctlMust(t, "deploy", "--yes", "-aa=a", "-ab=b")

	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-a-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-b-test-chart2")
	assertConfigMapExists(t, k, p.TestSlug()+"-a", "test-helm-ns-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug()+"-b", "test-helm-ns-test-chart1")
}

func TestHelmRenderOfflineKubernetes(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	stdout, _ := p.KluctlMust(t, "render", "--print-all", "--offline-kubernetes")
	cm1 := uo.FromStringMust(stdout)

	assert.Equal(t, map[string]any{
		"a":           "v1",
		"b":           "v2",
		"version":     "0.1.0",
		"kubeVersion": "v1.20.0",
	}, cm1.Object["data"])

	stdout, _ = p.KluctlMust(t, "render", "--print-all", "--offline-kubernetes", "--kubernetes-version", "1.22.1")
	cm1 = uo.FromStringMust(stdout)

	assert.Equal(t, map[string]any{
		"a":           "v1",
		"b":           "v2",
		"version":     "0.1.0",
		"kubeVersion": "v1.22.1",
	}, cm1.Object["data"])
}

func TestHelmLocalChart(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.AddHelmDeployment("helm1", test_utils.NewHelmTestRepoLocal("../test-chart1"), "", "", "test-helm-1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", test_utils.NewHelmTestRepoLocal("test-chart2"), "", "", "test-helm-2", p.TestSlug(), nil)

	test_utils.CreateHelmDir(t, "test-chart1", "0.1.0", filepath.Join(p.LocalProjectDir(), "test-chart1"))
	test_utils.CreateHelmDir(t, "test-chart2", "0.1.0", filepath.Join(p.LocalProjectDir(), "helm2/test-chart2"))

	p.KluctlMust(t, "deploy", "--yes")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-1-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-2-test-chart2")

	_, stderr := p.KluctlMust(t, "helm-pull")
	assert.NotContains(t, stderr, "test-chart1")
	assert.NotContains(t, stderr, "test-chart2")
}

func TestHelmSkipPrePull(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.1.1"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repo, "test-chart1", "0.1.1", "test-helm2", p.TestSlug(), nil)

	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	args := []string{"helm-pull"}
	_, stderr := p.KluctlMust(t, args...)
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.NotContains(t, stderr, "version 0.1.1")
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", repo.URL.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", repo.URL.Port())))

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(t, args...)
	assert.Contains(t, stderr, "Removing unused Chart with version 0.1.0")
	assert.NotContains(t, stderr, "version 0.1.1")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", repo.URL.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", repo.URL.Port())))

	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(false, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(t, args...)
	assert.Contains(t, stderr, "test-chart1: Pulling Chart with version 0.1.1")
	assert.NotContains(t, stderr, "version 0.1.0")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", repo.URL.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", repo.URL.Port())))

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(false, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(t, args...)
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.1")
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", repo.URL.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", repo.URL.Port())))

	// not try to update+pull
	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(t, args...)
	p.GitServer().CommitFiles("kluctl-project", []string{".helm-charts"}, false, ".helm-charts")
	args = []string{
		"helm-update",
		"--upgrade",
		"--commit",
	}
	_, stderr = p.KluctlMust(t, args...)
	assert.NotContains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.NotContains(t, stderr, "Pulling Chart with version 0.1.1")
	assert.Contains(t, stderr, "Pulling Chart with version 0.2.0")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", repo.URL.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", repo.URL.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.2.0", repo.URL.Port())))
}

func getChartDir(t *testing.T, p *test_project.TestProject, repo *test_utils.TestHelmRepo, chartName string, chartVersion string) string {
	var dir string

	switch repo.Type {
	case test_utils.TestHelmRepo_Helm:
		dir = filepath.Join(p.LocalProjectDir(), ".helm-charts", fmt.Sprintf("%s_%s_%s", repo.URL.Scheme, repo.URL.Port(), repo.URL.Hostname()), repo.URL.Path, chartName)
	case test_utils.TestHelmRepo_Oci:
		dir = filepath.Join(p.LocalProjectDir(), ".helm-charts", fmt.Sprintf("%s_%s_%s", repo.URL.Scheme, repo.URL.Port(), repo.URL.Hostname()), chartName)
	case test_utils.TestHelmRepo_Git:
		dir = filepath.Join(p.LocalProjectDir(), ".helm-charts", fmt.Sprintf("git_%s_%s_%s", repo.URL.Scheme, repo.URL.Port(), repo.URL.Hostname()), repo.URL.Path, chartName)
	}

	if chartVersion != "" {
		if repo.Type == test_utils.TestHelmRepo_Git {
			dir = filepath.Join(dir, "refs", "tags")
		}
		dir = filepath.Join(dir, chartVersion)
	}
	return dir
}

func getChartFile(t *testing.T, p *test_project.TestProject, repo *test_utils.TestHelmRepo, chartName string, chartVersion string) string {
	return filepath.Join(getChartDir(t, p, repo, chartName, chartVersion), "Chart.yaml")
}

func listChartVersions(t *testing.T, p *test_project.TestProject, repo *test_utils.TestHelmRepo, chartName string) []string {
	var versions []string
	if repo.Type == test_utils.TestHelmRepo_Git {
		dir := filepath.Join(getChartDir(t, p, repo, chartName, ""), "refs", "tags")
		des, err := os.ReadDir(dir)
		assert.NoError(t, err)

		for _, de := range des {
			versions = append(versions, de.Name())
		}
	} else {
		des, err := os.ReadDir(getChartDir(t, p, repo, chartName, ""))
		assert.NoError(t, err)

		for _, de := range des {
			versions = append(versions, de.Name())
		}
	}

	sort.Strings(versions)
	return versions
}

func TestHelmLookup(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	lookupCm := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "lookup-cm",
			Namespace: p.TestSlug(),
		},
		Data: map[string]string{
			"a": "lookupValue",
		},
	}
	err := k.Client.Create(context.Background(), &lookupCm)
	assert.NoError(t, err)

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	values1 := map[string]any{
		"lookup":          true,
		"lookupNamespace": lookupCm.Namespace,
		"lookupName":      lookupCm.Name,
	}

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), values1)

	p.KluctlMust(t, "helm-pull")
	p.KluctlMust(t, "deploy", "--yes")

	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	assertNestedFieldEquals(t, cm1, "lookupValue", "data", "lookup")

	s, _ := p.KluctlMust(t, "render", "--print-all")
	y, err := uo.FromString(s)
	assert.NoError(t, err)
	assertNestedFieldEquals(t, y, "lookupValue", "data", "lookup")

	s, _ = p.KluctlMust(t, "render", "--print-all", "--offline-kubernetes")
	y, err = uo.FromString(s)
	assert.NoError(t, err)
	assertNestedFieldEquals(t, y, "lookupReturnedNil", "data", "lookup")
}

func TestHelmWithTarget(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)
	p.UpdateTarget("test", nil)

	createNamespace(t, k, p.TestSlug())

	charts := []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}
	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Helm, "", charts)
	repo.Start(t)

	p.AddHelmDeployment("helm1", repo, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	p.KluctlMust(t, "helm-pull")
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")

	assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
}
