package k8s

import (
	"context"
	"fmt"

	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsListGVK(gvk schema.GroupVersionKind) bool {
	listGvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "List"}
	return gvk == listGvk
}

func FixNamespace(o *uo.UnstructuredObject, namespaced bool, def string) {
	ref := o.GetK8sRef()
	if !namespaced && ref.Namespace != "" {
		o.SetK8sNamespace("")
	} else if namespaced && ref.Namespace == "" {
		o.SetK8sNamespace(def)
	}
}

func FixNamespaceInRef(ref k8s.ObjectRef, namespaced bool, def string) k8s.ObjectRef {
	if !namespaced && ref.Namespace != "" {
		ref.Namespace = ""
	} else if namespaced && ref.Namespace == "" {
		ref.Namespace = def
	}
	return ref
}

func GetClusterId(ctx context.Context, c client.Client) (string, error) {
	var clusterId string

	// we reuse the kube-system namespace uid as global cluster id
	var ns corev1.Namespace
	err := c.Get(ctx, client.ObjectKey{Name: "kube-system"}, &ns)
	if err != nil {
		if !errors.IsForbidden(err) {
			return "", err
		}
		// There is a good chance that the user does not have permissions to GET specific namespaces while still being
		// able to list them. Let's give this a try.
		var nsl metav1.PartialObjectMetadataList
		nsl.SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "Namespace",
		})

		err = c.List(ctx, &nsl)
		if err != nil {
			return "", err
		}
		for _, x := range nsl.Items {
			if x.Name == "kube-system" {
				clusterId = string(x.UID)
			}
		}
	} else {
		clusterId = string(ns.UID)
	}
	if clusterId == "" {
		return "", fmt.Errorf("kube-system namespace has no uid")
	}
	return clusterId, nil
}

func GetSingleSecret(ctx context.Context, c client.Client, name string, namespace string, key string) (string, error) {
	return doGetOrGenerateSingleSecret(ctx, c, name, namespace, key, false, "")
}

func GetOrGenerateSingleSecret(ctx context.Context, c client.Client, name string, namespace string, key string, manager string) (string, error) {
	return doGetOrGenerateSingleSecret(ctx, c, name, namespace, key, true, manager)
}

func doGetOrGenerateSingleSecret(ctx context.Context, c client.Client, name string, namespace string, key string, allowGenerate bool, manager string) (string, error) {
	var secret corev1.Secret
	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &secret)
	if err != nil {
		if !allowGenerate || !errors.IsNotFound(err) {
			return "", err
		}
		secret.Name = name
		secret.Namespace = namespace
		err = c.Create(ctx, &secret, client.FieldOwner(manager))
		if err != nil && !errors.IsAlreadyExists(err) {
			return "", err
		}
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	x, ok := secret.Data[key]
	if ok {
		return string(x), nil
	}
	if !allowGenerate {
		return "", fmt.Errorf("secrets %s has no key %s", name, key)
	}

	generated := rand.String(32)
	patch := client.MergeFrom(secret.DeepCopy())
	secret.Data[key] = []byte(generated)

	err = c.Patch(ctx, &secret, patch, client.FieldOwner(manager))
	if err != nil {
		return "", err
	}

	return generated, nil
}
