package utils

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestRemoteObjectUtils_PermissionErrors(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
	expectedErr := errors.NewForbidden(gvr.GroupResource(), "secret", nil)

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "cm1", Namespace: "ns", Labels: map[string]string{"label1": "value1"}},
		Data: map[string][]byte{
			"test": []byte(`test`),
		},
	}

	f := k8s.NewFakeClientFactory(secret)
	f.AddError(gvr, "", "", expectedErr)
	k, err := k8s.NewK8sCluster(context.TODO(), f, false)
	if err != nil {
		t.Fatal(err)
	}

	dew := NewDeploymentErrorsAndWarnings()
	u := NewRemoteObjectsUtil(context.Background(), dew)
	discriminator := "d"
	err = u.UpdateRemoteObjects(k, &discriminator, []k8s2.ObjectRef{
		k8s2.NewObjectRef("", "v1", "Secret", "secret", "default"),
	}, false)
	assert.NoError(t, err)
	assert.Equal(t, []result.DeploymentError{{
		Message: "at least one permission error was encountered while gathering objects by discriminator labels. This might result in orphan object detection to not work properly"},
	}, dew.GetWarningsList())
	assert.Empty(t, dew.GetErrorsList())
}
