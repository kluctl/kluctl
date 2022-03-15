package utils

import (
	"github.com/codablock/kluctl/pkg/deployment"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
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
			lo:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			ro:   nil,
			ao:   []*uo.UnstructuredObject{newTestConfigMap("test", map[string]interface{}{})},
			a: func(t *testing.T, dtc *diffTestConfig) {
				assert.Len(t, dtc.du.NewObjects, 1)
				assert.Len(t, dtc.du.ChangedObjects, 0)
				assert.Equal(t, dtc.lo[0], dtc.du.NewObjects[0].Object)
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
