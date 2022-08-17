package flux

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// TODO wait for object to reconcile
// TODO add better error handling
func Patch(client dynamic.Interface,
	namespace string,
	objectName string,
	resource schema.GroupVersionResource,
	payload interface{}) error {

	patchBytes, _ := json.Marshal(payload)
	_, err := client.Resource(resource).
		Namespace(namespace).
		Patch(context.TODO(), objectName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	HandleError(err, objectName, namespace)

	return err
}

func GetSource(objectData *unstructured.Unstructured) (string, string) {
	spec := objectData.DeepCopy().Object["spec"]
	m, ok := spec.(map[string]interface{})["sourceRef"]
	if !ok {
		fmt.Printf("want type map[string]interface{};  got %T", spec)
		os.Exit(1)
	}

	sourceName := fmt.Sprintf("%v", m.(map[string]interface{})["name"])
	sourceNamespace := fmt.Sprintf("%v", m.(map[string]interface{})["namespace"])

	return sourceName, sourceNamespace
}

func GetObject(client dynamic.Interface,
	namespace string,
	objectName string,
	resource schema.GroupVersionResource) (*unstructured.Unstructured, error) {

	objectData, err := client.Resource(resource).Namespace(namespace).Get(context.TODO(), objectName, metav1.GetOptions{})
	HandleError(err, objectName, namespace)

	return objectData, err
}

func HandleError(err error, name string, namespace string) {
	if errors.IsNotFound(err) {
		fmt.Printf("✗ KluctlDeployment %s in namespace %s not found\n", name, namespace)
		os.Exit(1)
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("✗ Error getting kluctldeployment %s in namespace %s: %v\n",
			name, namespace, statusError.ErrStatus.Message)
		os.Exit(1)
	} else if err != nil {
		panic(err.Error())
	} else {
		fmt.Printf("► Found KluctlDeployment %s in namespace %s\n", name, namespace)
	}
}
