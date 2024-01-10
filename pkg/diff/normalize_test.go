package diff

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	"testing"
)

func buildObject(j ...string) *uo.UnstructuredObject {
	o := uo.FromMap(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "ns",
		},
	})

	for _, x := range j {
		o2 := uo.FromStringMust(x)
		o.Merge(o2)
	}
	return o
}

func buildResultObject(j ...string) *uo.UnstructuredObject {
	o := buildObject(`{"metadata": {"labels": {}, "annotations": {}}}`)
	for _, x := range j {
		o2 := uo.FromStringMust(x)
		o.Merge(o2)
	}
	return o
}

type testCase struct {
	remote         *uo.UnstructuredObject
	local          *uo.UnstructuredObject
	result         *uo.UnstructuredObject
	ignoreForDiffs []types.IgnoreForDiffItemConfig
}

func runTests(t *testing.T, tests []testCase) {
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r, err := NormalizeObject(tc.remote, tc.ignoreForDiffs, tc.local)
			if err != nil {
				t.Error(err)
			} else {
				rj, _ := yaml.WriteJsonString(r)
				ej, _ := yaml.WriteJsonString(tc.result)
				assert.Equal(t, ej, rj)
			}
		})
	}
}

func TestNormalizeNoop(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(), local: buildObject(), result: buildResultObject()},
	}
	runTests(t, testCases)
}

func TestNormalizeMetadata(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(`{"metadata": {"labels": null, "annotations": null}}`), local: buildObject(), result: buildResultObject()},
		{remote: buildObject(`{"metadata": {"managedFields": {}, "creationTimestamp": "test", "generation": "test", "resourceVersion": 123, "selfLink": "test", "uid": "test", "good": "keep"}}`), local: buildObject(), result: buildResultObject(`{"metadata": {"good": "keep"}}`)},
		{remote: buildObject(`{"metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "test", "good": "keep"}}}`), local: buildObject(), result: buildResultObject(`{"metadata": {"annotations": {"good": "keep"}}}`)},
	}
	runTests(t, testCases)
}

func TestNormalizeMisc(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(`{"spec": {"template": {"metadata": {"labels": {"controller-uid": "test", "good": "keep"}}}}, "selector": {"controller-uid": "test", "good": "keep"}}`), local: buildObject(), result: buildResultObject(`{"metadata":{"annotations":{},"labels":{}},"selector":{"controller-uid":"test","good":"keep"},"spec":{"template":{"metadata":{"labels":{"good":"keep"}}}}}`)},
		{remote: buildObject(`{"status": {"test": "test"}}`), local: buildObject(), result: buildResultObject()},
	}
	runTests(t, testCases)
}

func TestNormalizeContainers(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(`{"spec": {"template": {"spec": {"containers": ["env": [{"name": "a", "value": "x"}]]}}}}`), local: buildObject(), result: buildResultObject(`{"spec": {"template": {"spec": {"containers": ["env": {"a":{"name":"a","value":"x"}}]}}}}`)},
	}
	runTests(t, testCases)
}

func TestNormalizeData(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(`{"apiVersion": "v1", "kind": "ConfigMap", "data": {"good": "keep"}}`), local: buildObject(), result: buildResultObject(`{"apiVersion": "v1", "kind": "ConfigMap", "data": {"good": "keep"}}`)},
		{remote: buildObject(`{"apiVersion": "v1", "kind": "ConfigMap", "data": {}}`), local: buildObject(), result: buildResultObject(`{"apiVersion": "v1", "kind": "ConfigMap"}`)},
		{remote: buildObject(`{"apiVersion": "v1", "kind": "ConfigMap", "data": null}`), local: buildObject(), result: buildResultObject(`{"apiVersion": "v1", "kind": "ConfigMap"}`)},
		{remote: buildObject(`{"apiVersion": "v1", "kind": "ConfigMap"}`), local: buildObject(), result: buildResultObject(`{"apiVersion": "v1", "kind": "ConfigMap"}`)},
	}
	runTests(t, testCases)
}

func TestNormalizeServiceAccounts(t *testing.T) {
	testCases := []testCase{
		{remote: buildObject(`{"apiVersion": "v1", "kind": "ServiceAccount", "secrets": [{"name": "test-remove"},{"name": "good"}]}`), local: buildObject(), result: buildResultObject(`{"apiVersion": "v1", "kind": "ServiceAccount", "secrets": [{"name": "good"}]}`)},
	}
	runTests(t, testCases)
}

func TestNormalizeIgnoreForDiffs(t *testing.T) {
	testCases := []testCase{
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.l1", "metadata.labels.l2"}},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPathRegex: []string{`metadata\.labels\.l.*`}},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}, Group: utils.StrPtr("Nope")},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}, Kind: utils.StrPtr("Nope")},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}, Name: utils.StrPtr("Nope")},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}, Namespace: utils.StrPtr("Nope")},
			},
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2"}}}`),
			local:  buildObject(),
			result: buildResultObject(`{"metadata": {"labels": {}}}`),
			ignoreForDiffs: []types.IgnoreForDiffItemConfig{
				{FieldPath: []string{"metadata.labels.*"}, Group: utils.StrPtr("apps"), Kind: utils.StrPtr("Deployment"), Name: utils.StrPtr("test"), Namespace: utils.StrPtr("ns")},
			},
		},
	}
	runTests(t, testCases)
}

func TestNormalizeIgnoreForDiffsByAnnotations(t *testing.T) {
	testCases := []testCase{
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "good": "keep"}}}`),
			local:  buildObject(`{"metadata": {"annotations": {"kluctl.io/ignore-diff": "true"}}}`),
			result: uo.FromStringMust(`{}`),
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2"}}}`),
			local:  buildObject(`{"metadata": {"annotations": {"kluctl.io/ignore-diff-field": "metadata.labels.l1"}}}`),
			result: buildResultObject(`{"metadata": {"labels": {"l2": "l2"}}}`),
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "l3": "l3", "good": "keep"}}}`),
			local:  buildObject(`{"metadata": {"annotations": {"kluctl.io/ignore-diff-field": "metadata.labels.l1", "kluctl.io/ignore-diff-field-1": "metadata.labels.l2", "kluctl.io/ignore-diff-field-3": "metadata.labels.l3"}}}`),
			result: buildResultObject(`{"metadata": {"labels": {"good": "keep"}}}`),
		},
		{
			remote: buildObject(`{"metadata": {"labels": {"l1": "v1", "l2": "l2", "l3": "l3", "good": "keep"}}}`),
			local:  buildObject(`{"metadata": {"annotations": {"kluctl.io/ignore-diff-field-regex": "metadata\\.labels\\.l[12]", "kluctl.io/ignore-diff-field-regex-1": "metadata\\.labels\\.l3"}}}`),
			result: buildResultObject(`{"metadata": {"labels": {"good": "keep"}}}`),
		},
	}
	runTests(t, testCases)
}
