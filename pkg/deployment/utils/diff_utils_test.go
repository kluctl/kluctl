package utils

import (
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"testing"
)

type diffTestConfig struct {
	name string
	lo   []*uo.UnstructuredObject
	ro   []*uo.UnstructuredObject
	ao   []*uo.UnstructuredObject
	a    func(t *testing.T, dtc *diffTestConfig)

	dew *DeploymentErrorsAndWarnings
	ru  *RemoteObjectUtils
	du  *diffUtil
}

func (dtc *diffTestConfig) newRemoteObjects(dew *DeploymentErrorsAndWarnings) *RemoteObjectUtils {
	ru := NewRemoteObjectsUtil(dew)
	for _, ro := range dtc.ro {
		ru.remoteObjects[ro.GetK8sRef()] = ro
	}
	return ru
}

func (dtc *diffTestConfig) newDeploymentItems() []*deployment.DeploymentItem {
	return []*deployment.DeploymentItem{
		{
			Objects: dtc.lo,
		},
	}
}

func (dtc *diffTestConfig) appliedObjectsMap() map[k8s2.ObjectRef]*uo.UnstructuredObject {
	ret := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, o := range dtc.ao {
		ret[o.GetK8sRef()] = o
	}
	return ret
}

func newTestConfigMap(name string, data map[string]interface{}) *uo.UnstructuredObject {
	o := uo.New()
	o.SetK8sGVKs("", "v1", "ConfigMap")
	o.SetK8sName(name)
	o.SetK8sNamespace("default")
	o.SetNestedField(data, "data")
	return o
}

func TestDiff(t *testing.T) {
	tests := []*diffTestConfig{
		{
			name: "One new object",
			ro:   nil,
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 1)
				assert.Len(t, dtc.du.ChangedObjects, 0)
				assert.Equal(t, dtc.lo[0], dtc.du.NewObjects[0].Object)
			},
		},
		{
			name: "One deleted object",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			lo:   nil,
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 0)
			},
		},
		{
			name: "One changed object (changed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, []types.Change{
					types.Change{Type: "update", JsonPath: "data.d1", OldValue: "v1", NewValue: "v2", UnifiedDiff: "-v1\n+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (new field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, []types.Change{
					types.Change{Type: "insert", JsonPath: "data.d2", OldValue: interface{}(nil), NewValue: "v2", UnifiedDiff: "+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (removed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, []types.Change{
					types.Change{Type: "delete", JsonPath: "data.d2", OldValue: "v2", NewValue: interface{}(nil), UnifiedDiff: "-v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (new + removed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d2": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d2": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, []types.Change{
					types.Change{Type: "delete", JsonPath: "data.d1", OldValue: "v1", NewValue: interface{}(nil), UnifiedDiff: "-v1"},
					types.Change{Type: "insert", JsonPath: "data.d2", OldValue: interface{}(nil), NewValue: "v2", UnifiedDiff: "+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (mixed changes)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v12", "d3": "v3"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v12", "d3": "v3"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 0)
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, []types.Change{
					types.Change{Type: "delete", JsonPath: "data.d2", OldValue: "v2", NewValue: interface{}(nil), UnifiedDiff: "-v2"},
					types.Change{Type: "insert", JsonPath: "data.d3", OldValue: interface{}(nil), NewValue: "v3", UnifiedDiff: "+v3"},
					types.Change{Type: "update", JsonPath: "data.d1", OldValue: "v1", NewValue: "v12", UnifiedDiff: "-v1\n+v12"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "Two changed objects + One new object",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test1", map[string]interface{}{"d1": "v1", "d2": "v2"}), newTestConfigMap("test2", map[string]interface{}{"xd1": "xv1", "xd2": "xv2"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test1", map[string]interface{}{"d1": "v1", "d2": "v3"}), newTestConfigMap("test2", map[string]interface{}{"xd3": "xv2"}), newTestConfigMap("test3", map[string]interface{}{"yd3": "yv2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test1", map[string]interface{}{"d1": "v1", "d2": "v3"}), newTestConfigMap("test2", map[string]interface{}{"xd3": "xv2"}), newTestConfigMap("test3", map[string]interface{}{"yd3": "yv2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 1)
				assert.Len(t, dtc.du.ChangedObjects, 2)
				assert.Equal(t, dtc.du.ChangedObjects[0].NewObject, dtc.ao[0])
				assert.Equal(t, dtc.du.ChangedObjects[1].NewObject, dtc.ao[1])
				assert.Equal(t, dtc.du.NewObjects[0].Object, dtc.ao[2])
				assert.Equal(t, []types.Change{
					types.Change{Type: "update", JsonPath: "data.d2", OldValue: "v2", NewValue: "v3", UnifiedDiff: "-v2\n+v3"},
				}, dtc.du.ChangedObjects[0].Changes)
				assert.Equal(t, []types.Change{
					types.Change{Type: "delete", JsonPath: "data.xd1", OldValue: "xv1", NewValue: interface{}(nil), UnifiedDiff: "-xv1"},
					types.Change{Type: "delete", JsonPath: "data.xd2", OldValue: "xv2", NewValue: interface{}(nil), UnifiedDiff: "-xv2"},
					types.Change{Type: "insert", JsonPath: "data.xd3", OldValue: interface{}(nil), NewValue: "xv2", UnifiedDiff: "+xv2"},
				}, dtc.du.ChangedObjects[1].Changes)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.dew = NewDeploymentErrorsAndWarnings()
			test.ru = test.newRemoteObjects(test.dew)
			test.du = NewDiffUtil(test.dew, test.newDeploymentItems(), test.ru, test.appliedObjectsMap())
			test.du.Diff()
			test.a(t, test)
		})
	}
}
