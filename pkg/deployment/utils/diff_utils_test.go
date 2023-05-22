package utils

import (
	"context"
	"encoding/json"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	du  *DiffUtil
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
	buildRaw := func(x any) *apiextensionsv1.JSON {
		if x == nil {
			return nil
		}
		b, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}
		return &apiextensionsv1.JSON{
			Raw: b,
		}
	}
	buildChange := func(typ string, jsonPath string, oldValue any, newValue any, unifiedDiff string) result.Change {
		return result.Change{
			Type:        typ,
			JsonPath:    jsonPath,
			NewValue:    buildRaw(newValue),
			OldValue:    buildRaw(oldValue),
			UnifiedDiff: unifiedDiff,
		}
	}

	tests := []*diffTestConfig{
		{
			name: "One changed object (changed field)",
			ro:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v1"})},
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{"d1": "v2"})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.ChangedObjects, 1)
				assert.Equal(t, []result.Change{
					buildChange("update", "data.d1", buildRaw("v1"), buildRaw("v2"), "-v1\n+v2"),
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
					buildChange("insert", "data.d2", nil, buildRaw("v2"), "+v2"),
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
					buildChange("delete", "data.d2", buildRaw("v2"), nil, "-v2"),
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
					buildChange("delete", "data.d1", buildRaw("v1"), nil, "-v1"),
					buildChange("insert", "data.d2", nil, buildRaw("v2"), "+v2"),
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
					buildChange("update", "data.d1", buildRaw("v1"), buildRaw("v12"), "-v1\n+v12"),
					buildChange("delete", "data.d2", buildRaw("v2"), nil, "-v2"),
					buildChange("insert", "data.d3", nil, buildRaw("v3"), "+v3"),
				}, dtc.du.ChangedObjects[0].Changes)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.dew = NewDeploymentErrorsAndWarnings()
			test.ru = test.newRemoteObjects(test.dew)
			test.du = NewDiffUtil(test.dew, test.ru, test.appliedObjectsMap())
			test.du.DiffDeploymentItems(test.newDeploymentItems())
			test.a(t, test)
		})
	}
}
