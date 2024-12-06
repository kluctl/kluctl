package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"path/filepath"
	"strings"
	"testing"
)

func TestWhen(t *testing.T) {
	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())
	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	type testCase struct {
		dir     string
		when    string
		whenInc string
		depInc  string
		want    bool
		name    string
	}

	tests := []*testCase{
		{dir: "cm_empty", when: "", want: true},
		{dir: "cm_true", when: "True", want: true},
		{dir: "cm_false", when: "False", want: false},
		{dir: "cm_eq", when: "args.a == 'test'", want: true},
		{dir: "cm_ne", when: "args.a != 'test'", want: false},
		{dir: "cm_eq2", when: "args.b == 'test'", want: false},
		{dir: "cm_ne2", when: "args.b != 'test'", want: true},
		{dir: "inc1/cm_empty", when: "", want: true},
		{dir: "inc2/cm_true", when: "True", want: true},
		{dir: "inc3/cm_false", when: "False", want: false},
		{dir: "inc4/cm_eq", when: "args.a == 'test'", want: true},
		{dir: "inc5/cm_ne", when: "args.a != 'test'", want: false},
		{dir: "inc_when1/cm_empty", whenInc: "", want: true},
		{dir: "inc_when2/cm_true", whenInc: "True", want: true},
		{dir: "inc_when3/cm_false", whenInc: "False", want: false},
		{dir: "inc_when4/cm_eq", whenInc: "args.a == 'test'", want: true},
		{dir: "inc_when5/cm_ne", whenInc: "args.a != 'test'", want: false},
		{dir: "dep_inc_when1/cm_empty", depInc: "", want: true},
		{dir: "dep_inc_when2/cm_true", depInc: "True", want: true},
		{dir: "dep_inc_when3/cm_false", depInc: "False", want: false},
		{dir: "dep_inc_when4/cm_eq", depInc: "args.a == 'test'", want: true},
		{dir: "dep_inc_when5/cm_ne", depInc: "args.a != 'test'", want: false},
	}

	for _, test := range tests {
		test.name = strings.ReplaceAll(test.dir, "/", "_")
		test.name = strings.ReplaceAll(test.name, "_", "-")
		addConfigMapDeployment(p, test.dir, nil, resourceOpts{
			name:      test.name,
			namespace: p.TestSlug(),
			when:      test.when,
		})
		if test.whenInc != "" {
			dir := filepath.Dir(test.dir)
			p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
				for _, it := range items {
					inc, _, _ := it.GetNestedString("include")
					if inc == dir {
						_ = it.SetNestedField(test.whenInc, "when")
						break
					}
				}
				return items
			})
		}
		if test.depInc != "" {
			dir := filepath.Dir(test.dir)
			p.UpdateDeploymentYaml(dir, func(o *uo.UnstructuredObject) error {
				_ = o.SetNestedField(test.depInc, "when")
				return nil
			})
		}
	}

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "-aa=test", "-ab=test2")

	for _, test := range tests {
		if test.want {
			assertConfigMapExists(t, k, p.TestSlug(), test.name)
		} else {
			assertConfigMapNotExists(t, k, p.TestSlug(), test.name)
		}
	}
}
