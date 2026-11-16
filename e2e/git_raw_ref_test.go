package e2e

import (
	"testing"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
)

// Keep the commit reachable only through the raw ref, so a normal branch/tag
// mirror cannot accidentally satisfy the test without fetching the raw ref.
func createRawRefCommit(t *testing.T, p *test_project.TestProject, ref string, update func()) string {
	t.Helper()
	commit := createBranchAndTag(t, p, "raw-ref-test", "", "", update)
	err := p.GetGitRepo().Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), plumbing.NewHash(commit)))
	assert.NoError(t, err)
	err = p.GetGitRepo().Storer.RemoveReference(plumbing.NewBranchReferenceName("raw-ref-test"))
	assert.NoError(t, err)
	return commit
}

func TestGitIncludeRawRef(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"refs/merge-requests/123/merge", "refs/pull/123/head"} {
		t.Run(ref, func(t *testing.T) {
			p := test_project.NewTestProject(t)
			ip := test_project.NewTestProject(t)
			createNamespace(t, defaultCluster1, p.TestSlug())
			p.UpdateTarget("test", nil)
			p.AddDeploymentItem("", uo.FromMap(map[string]any{
				"git": map[string]any{
					"url": ip.GitUrl(),
					"ref": map[string]any{"raw": ref},
				},
			}))

			setValue := func(value string) string {
				return createRawRefCommit(t, ip, ref, func() {
					addConfigMapDeployment(ip, "cm", map[string]string{"value": value}, resourceOpts{
						name: "raw-ref", namespace: p.TestSlug(),
					})
				})
			}
			deployAndCheck := func(value string) {
				p.KluctlMust(t, "deploy", "--yes", "-t", "test")
				cm := assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "raw-ref")
				assertNestedFieldEquals(t, cm, value, "data", "value")
			}

			firstCommit := setValue("first")
			deployAndCheck("first")
			// The second commit diverges from the first, as with a regenerated MR merge ref.
			setValue("second")
			deployAndCheck("second")
			err := ip.GetGitRepo().Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), plumbing.NewHash(firstCommit)))
			assert.NoError(t, err)
			deployAndCheck("first")
		})
	}
}

func TestGitIncludeRawRefErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		ref     map[string]any
		message string
	}{
		{"missing", map[string]any{"raw": "refs/merge-requests/404/merge"}, "refs/merge-requests/404/merge"},
		{"not a full ref", map[string]any{"raw": "merge-requests/123/merge"}, "raw must be a full ref name starting with 'refs/'"},
		{"raw and branch", map[string]any{"raw": "refs/pull/123/head", "branch": "master"}, "only one of the ref fields can be set"},
		{"raw and tag", map[string]any{"raw": "refs/pull/123/head", "tag": "v1"}, "only one of the ref fields can be set"},
		{"raw and commit", map[string]any{"raw": "refs/pull/123/head", "commit": "0123456789012345678901234567890123456789"}, "only one of the ref fields can be set"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := test_project.NewTestProject(t)
			ip := test_project.NewTestProject(t)
			p.AddDeploymentItem("", uo.FromMap(map[string]any{
				"git": map[string]any{"url": ip.GitUrl(), "ref": tt.ref},
			}))
			_, _, err := p.Kluctl(t, "render", "--print-all")
			assert.ErrorContains(t, err, tt.message)
		})
	}
}
