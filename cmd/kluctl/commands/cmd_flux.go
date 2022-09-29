package commands

import (
	"context"
	"fmt"
	"time"

	kube "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type fluxCmd struct {
	Reconcile fluxReconcileCmd `cmd:"" help:"Reconcile KluctlDeployment"`
	Suspend   fluxSuspendCmd   `cmd:"" help:"Suspend KluctlDeployment"`
	Resume    fluxResumeCmd    `cmd:"" help:"Resume KluctlDeployment"`
}

func WaitForReady(k *kube.K8sCluster, ref k8s.ObjectRef) (bool, error) {
	o, _, err := k.GetSingleObject(ref)
	if o == nil {
		return false, err
	}
	retry := 0
	finalStatus := true
	var errorMsg error
	for s, err := GetObjectStatus(o); s != true; {
		if retry >= 5 || err != nil {
			finalStatus = false
			errorMsg = err
			break
		}
		retry++
		time.Sleep(8 * time.Second)
	}
	return finalStatus, errorMsg
}

func GetObjectStatus(o *uo.UnstructuredObject) (bool, error) {
	conditions, ok, err := o.GetNestedField("status", "conditions")
	if !ok {
		return false, err
	}
	for _, v := range conditions.([]interface{}) {
		if (v.(map[string]interface{})["type"]) == "Ready" {
			return true, nil
		}
	}

	return false, err
}

func GetObjectSource(o *uo.UnstructuredObject, name string, namespace string, ctx context.Context) (string, string, error) {
	status.Trace(ctx, "fetching %s in %s", name, namespace)

	sourceName, ok, err := o.GetNestedField("spec", "sourceRef", "name")
	if !ok {
		return "", "", err
	}

	sourceNamespace, ok, err := o.GetNestedField("spec", "sourceRef", "namespace")
	if !ok {
		return "", "", err
	}

	return fmt.Sprintf("%v", sourceName),
		fmt.Sprintf("%v", sourceNamespace),
		err
}
