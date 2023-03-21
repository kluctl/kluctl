package utils

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
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
	ru := NewRemoteObjectsUtil(context.TODO(), dew)
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
			name: "One changed object (changed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					result.Change{Type: "update", JsonPath: "data.d1", OldValue: "v1", NewValue: "v2", UnifiedDiff: "-v1\n+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (new field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					result.Change{Type: "insert", JsonPath: "data.d2", OldValue: interface{}(nil), NewValue: "v2", UnifiedDiff: "+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (removed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					result.Change{Type: "delete", JsonPath: "data.d2", OldValue: "v2", NewValue: interface{}(nil), UnifiedDiff: "-v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (new + removed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d2": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d2": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					result.Change{Type: "delete", JsonPath: "data.d1", OldValue: "v1", NewValue: interface{}(nil), UnifiedDiff: "-v1"},
					result.Change{Type: "insert", JsonPath: "data.d2", OldValue: interface{}(nil), NewValue: "v2", UnifiedDiff: "+v2"},
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
		{
			name: "One changed object (mixed changes)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1", "d2": "v2"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v12", "d3": "v3"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v12", "d3": "v3"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					result.Change{Type: "update", JsonPath: "data.d1", OldValue: "v1", NewValue: "v12", UnifiedDiff: "-v1\n+v12"},
					result.Change{Type: "delete", JsonPath: "data.d2", OldValue: "v2", NewValue: interface{}(nil), UnifiedDiff: "-v2"},
					result.Change{Type: "insert", JsonPath: "data.d3", OldValue: interface{}(nil), NewValue: "v3", UnifiedDiff: "+v3"},
				}, dtc.du.ChangedObjects[0].Changes)
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
