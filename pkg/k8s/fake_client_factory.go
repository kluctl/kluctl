package k8s

import (
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	fake_dynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

type fakeClientFactory struct {
	clientSet     *fake.Clientset
	dynamicClient *fake_dynamic.FakeDynamicClient
}

func (f *fakeClientFactory) RESTConfig() *rest.Config {
	return nil
}

func (f *fakeClientFactory) GetCA() []byte {
	return []byte{}
}

func (f *fakeClientFactory) CloseIdleConnections() {
}

func (f *fakeClientFactory) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	return f.clientSet.Discovery(), nil
}

func (f *fakeClientFactory) CoreV1Client(wh rest.WarningHandler) (corev1.CoreV1Interface, error) {
	return f.clientSet.CoreV1(), nil
}

func (f *fakeClientFactory) DynamicClient(wh rest.WarningHandler) (dynamic.Interface, error) {
	return f.dynamicClient, nil
}

func NewFakeClientFactory(objects ...runtime.Object) *fakeClientFactory {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	clientSet := fake.NewSimpleClientset(objects...)

	clientSet.Fake.Resources = ConvertSchemeToAPIResources(scheme)

	dynamicClient := fake_dynamic.NewSimpleDynamicClient(scheme, objects...)

	return &fakeClientFactory{
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
	}
}

type HasName interface {
	GetName() string
}

func (f *fakeClientFactory) AddError(gvr schema.GroupVersionResource, name string, namespace string, retErr error) {
	f.dynamicClient.PrependReactor("*", gvr.Resource, func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		if namespace != "" && namespace != action.GetNamespace() {
			return false, nil, nil
		}
		switch a := action.(type) {
		case HasName:
			if name != "" && name != a.GetName() {
				return false, nil, nil
			}
			return true, nil, retErr
		default:
			return true, nil, retErr
		}
	})
}

func ConvertSchemeToAPIResources(s *runtime.Scheme) []*metav1.APIResourceList {
	m := map[schema.GroupVersion][]metav1.APIResource{}
	var allTypes []schema.GroupVersionKind
	listTypes := map[schema.GroupVersionKind]bool{}
	for gvk, _ := range s.AllKnownTypes() {
		if gvk.Version == "__internal" {
			continue
		}
		if strings.HasSuffix(gvk.Kind, "List") {
			listTypes[gvk] = true
		}
		allTypes = append(allTypes, gvk)
	}

	for _, gvk := range allTypes {
		if strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		// we misuse kyaml here
		n, _ := openapi.IsNamespaceScoped(yaml.TypeMeta{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
		})

		verbs := []string{"delete", "deletecollection", "get", "patch", "create", "update", "watch"}
		if listTypes[schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind + "List",
		}] {
			verbs = append(verbs, "list")
		}

		singularGvr, _ := meta.UnsafeGuessKindToResource(gvk)

		ar := metav1.APIResource{
			Name:       singularGvr.Resource,
			Namespaced: n,
			Group:      gvk.Group,
			Version:    gvk.Version,
			Kind:       gvk.Kind,
			Verbs:      verbs,
		}

		l, _ := m[gvk.GroupVersion()]
		l = append(l, ar)
		m[gvk.GroupVersion()] = l
	}

	var ret []*metav1.APIResourceList
	for gv, arl := range m {
		ret = append(ret, &metav1.APIResourceList{
			GroupVersion: gv.String(),
			APIResources: arl,
		})
	}

	return ret
}
