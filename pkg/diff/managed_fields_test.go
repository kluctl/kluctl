package diff

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"testing"
)

func TestResolveFieldManagerConflicts(t *testing.T) {
	type testCase struct {
		name   string
		remote *uo.UnstructuredObject
		local  *uo.UnstructuredObject
		status metav1.Status
		result *uo.UnstructuredObject
		lost   []LostOwnership
		anns   map[string]string
	}

	type fieldInfo struct {
		name    string
		value   string
		manager string
	}

	buildConfigMap := func(fields ...fieldInfo) *uo.UnstructuredObject {
		o := uo.FromMap(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "name",
				"namespace": "namespace",
			},
			"data": map[string]any{},
		})
		pathesByManagers := map[string][]fieldpath.Path{}
		for _, fi := range fields {
			_ = o.SetNestedField(fi.value, "data", fi.name)
			if fi.manager != "" {
				pathesByManagers[fi.manager] = append(pathesByManagers[fi.manager], fieldpath.MakePathOrDie("data", fi.name))
			}
		}
		var managedFields []any
		for manager, pbm := range pathesByManagers {
			fs := fieldpath.NewSet(pbm...)
			json, _ := fs.ToJSON()
			fsY, _ := uo.FromString(string(json))
			managedFields = append(managedFields, map[string]interface{}{
				"apiVersion": "v1",
				"fieldsType": "FieldsV1",
				"manager":    manager,
				"operation":  "Apply",
				"time":       "dummy",
				"fieldsV1":   fsY.Object,
			})
		}
		_ = o.SetNestedField(managedFields, "metadata", "managedFields")
		return o
	}

	buildConflicts := func(fields ...string) metav1.Status {
		var s metav1.Status
		s.Details = &metav1.StatusDetails{}
		for _, f := range fields {
			s.Details.Causes = append(s.Details.Causes, metav1.StatusCause{
				Type:  metav1.CauseTypeFieldManagerConflict,
				Field: fmt.Sprintf(".data.%s", f),
			})
		}
		return s
	}

	buildLost := func(fields ...string) []LostOwnership {
		var l []LostOwnership
		for _, f := range fields {
			l = append(l, LostOwnership{
				Field:   fmt.Sprintf(".data.%s", f),
				Message: "",
			})
		}
		return l
	}

	buildAnnotations := func(kvs ...string) map[string]string {
		ret := map[string]string{}
		for i := 0; i < len(kvs); i += 2 {
			ret[kvs[i]] = kvs[i+1]
		}
		return ret
	}

	tests := []testCase{
		{
			name:   "overwrite-kubectl",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "kubectl"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}),
			status: buildConflicts("d1"),
			result: buildConfigMap(fieldInfo{"d1", "x", "m1"}),
		},
		{
			name:   "lost-field",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}),
			status: buildConflicts("d1"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost("d1"),
			// also test non-matching fields here
			anns: buildAnnotations("kluctl.io/force-apply-field", "data.d3"),
		},
		{
			name:   "force-apply-object",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/force-apply", "true"),
		},
		{
			name:   "force-apply-field",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost("d3"),
			anns:   buildAnnotations("kluctl.io/force-apply-field", "data.d1"),
		},
		{
			name:   "force-apply-field-xxx",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			lost:   buildLost("d1"),
			anns:   buildAnnotations("kluctl.io/force-apply-field-123", "data.d3"),
		},
		{
			name:   "ignore-conflicts",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/ignore-conflicts", "true"),
		},
		{
			name:   "ignore-conflicts-field",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost("d3"),
			anns:   buildAnnotations("kluctl.io/ignore-conflicts-field", "data.d1"),
		},
		{
			name:   "ignore-conflicts-field-xxx",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost("d1"),
			anns:   buildAnnotations("kluctl.io/ignore-conflicts-field-123", "data.d3"),
		},
		{
			name:   "force-apply-object-ignore-conflicts",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/force-apply", "true", "kluctl.io/ignore-conflicts", "true"),
		},
		{
			name:   "force-apply-object-ignore-conflicts-field",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/force-apply", "true", "kluctl.io/ignore-conflicts-field", "data.d1"),
		},
		{
			name:   "force-apply-field-ignore-conflicts",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/force-apply-field", "data.d1", "kluctl.io/ignore-conflicts", "true"),
		},
		{
			name:   "force-apply-field-ignore-conflicts-field",
			remote: buildConfigMap(fieldInfo{"d1", "v1", "c1"}, fieldInfo{"d2", "v2", "m1"}, fieldInfo{"d3", "v3", "c1"}),
			local:  buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}, fieldInfo{"d3", "x", "m1"}),
			status: buildConflicts("d1", "d3"),
			result: buildConfigMap(fieldInfo{"d1", "x", "m1"}, fieldInfo{"d2", "x", "m1"}),
			lost:   buildLost(),
			anns:   buildAnnotations("kluctl.io/force-apply-field", "data.d1", "kluctl.io/ignore-conflicts-field", "data.d3"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.local != nil {
				_ = tc.local.RemoveNestedField("metadata", "managedFields")
			}
			if tc.result != nil {
				_ = tc.result.RemoveNestedField("metadata", "managedFields")
			}
			for k, v := range tc.anns {
				if tc.local != nil {
					tc.local.SetK8sAnnotation(k, v)
				}
				if tc.result != nil {
					tc.result.SetK8sAnnotation(k, v)
				}
			}

			cr := ConflictResolver{}
			r, l, err := cr.ResolveConflicts(tc.local, tc.remote, tc.status)
			assert.NoError(t, err)
			assert.Equal(t, tc.result, r)
			assert.Equal(t, tc.lost, l)
		})
	}
}
