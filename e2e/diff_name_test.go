package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sort"
	"testing"
)

func TestDiffName(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", map[string]string{
		"d1": "{{ args.v }}",
	}, resourceOpts{
		name:      "{{ args.cm_name }}",
		fname:     "cm.yaml",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"kluctl.io/diff-name": "cm",
		},
	})
	p.KluctlMust("deploy", "--yes", "-t", "test", "-a", "cm_name=cm-1", "-a", "v=a")
	assertConfigMapExists(t, k, p.TestSlug(), "cm-1")

	resultStr, _ := p.KluctlMust("deploy", "--yes", "-t", "test", "-a", "cm_name=cm-2", "-a", "v=b", "-oyaml")
	assertConfigMapExists(t, k, p.TestSlug(), "cm-2")

	var cr result.CompactedCommandResult
	err := yaml.ReadYamlString(resultStr, &cr)
	assert.NoError(t, err)

	r := cr.ToNonCompacted()
	sort.Slice(r.Objects, func(i, j int) bool {
		return r.Objects[i].Ref.String() < r.Objects[j].Ref.String()
	})

	assert.Len(t, r.Objects, 2)
	assert.Equal(t, result.BaseObject{
		Ref: k8s.ObjectRef{Version: "v1", Kind: "ConfigMap", Name: "cm", Namespace: p.TestSlug()},
		Changes: []result.Change{
			{Type: "update", JsonPath: "data.d1", OldValue: &v1.JSON{Raw: []byte("\"a\"")}, NewValue: &v1.JSON{Raw: []byte("\"b\"")}, UnifiedDiff: "-a\n+b"},
			{Type: "update", JsonPath: "metadata.name", OldValue: &v1.JSON{Raw: []byte("\"cm-1\"")}, NewValue: &v1.JSON{Raw: []byte("\"cm-2\"")}, UnifiedDiff: "-cm-1\n+cm-2"}},
	}, r.Objects[0].BaseObject)
	assert.Equal(t, result.BaseObject{
		Ref:    k8s.ObjectRef{Version: "v1", Kind: "ConfigMap", Name: "cm-1", Namespace: p.TestSlug()},
		Orphan: true,
	}, r.Objects[1].BaseObject)

	resultStr, _ = p.KluctlMust("deploy", "--yes", "-t", "test", "-a", "cm_name=cm-3", "-a", "v=c", "-oyaml")
	assertConfigMapExists(t, k, p.TestSlug(), "cm-2")
	err = yaml.ReadYamlString(resultStr, &cr)
	assert.NoError(t, err)

	r = cr.ToNonCompacted()
	sort.Slice(r.Objects, func(i, j int) bool {
		return r.Objects[i].Ref.String() < r.Objects[j].Ref.String()
	})

	assert.Len(t, r.Objects, 3)
	assert.Equal(t, result.BaseObject{
		Ref: k8s.ObjectRef{Version: "v1", Kind: "ConfigMap", Name: "cm", Namespace: p.TestSlug()},
		Changes: []result.Change{
			{Type: "update", JsonPath: "data.d1", OldValue: &v1.JSON{Raw: []byte("\"b\"")}, NewValue: &v1.JSON{Raw: []byte("\"c\"")}, UnifiedDiff: "-b\n+c"},
			{Type: "update", JsonPath: "metadata.name", OldValue: &v1.JSON{Raw: []byte("\"cm-2\"")}, NewValue: &v1.JSON{Raw: []byte("\"cm-3\"")}, UnifiedDiff: "-cm-2\n+cm-3"}},
	}, r.Objects[0].BaseObject)
	assert.Equal(t, result.BaseObject{
		Ref:    k8s.ObjectRef{Version: "v1", Kind: "ConfigMap", Name: "cm-1", Namespace: p.TestSlug()},
		Orphan: true,
	}, r.Objects[1].BaseObject)
	assert.Equal(t, result.BaseObject{
		Ref:    k8s.ObjectRef{Version: "v1", Kind: "ConfigMap", Name: "cm-2", Namespace: p.TestSlug()},
		Orphan: true,
	}, r.Objects[2].BaseObject)
}
