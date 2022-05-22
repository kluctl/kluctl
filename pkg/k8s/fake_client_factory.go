package k8s

import (
	"github.com/kluctl/kluctl/v2/pkg/utils"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	fake_dynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	metadata_fake "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

type fakeClientFactory struct {
	clientSet *fake.Clientset
	objects   []runtime.Object
	scheme    *runtime.Scheme
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
	return fake_dynamic.NewSimpleDynamicClient(f.scheme, f.objects...), nil
}

func (f *fakeClientFactory) MetadataClient(wh rest.WarningHandler) (metadata.Interface, error) {
	return metadata_fake.NewSimpleMetadataClient(f.scheme, f.objects...), nil
}

func NewFakeClientFactory(objects ...runtime.Object) ClientFactory {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	clientSet := fake.NewSimpleClientset(objects...)

	clientSet.Fake.Resources = ConvertSchemeToAPIResources(scheme)

	return &fakeClientFactory{
		clientSet: clientSet,
		objects:   objects,
		scheme:    scheme,
	}
}

func ConvertSchemeToAPIResources(s *runtime.Scheme) []*metav1.APIResourceList {
	utils.WaitForOpenapiInitDone()

	m := map[schema.GroupVersion][]metav1.APIResource{}

	for gvk, _ := range s.AllKnownTypes() {
		// we misuse kyaml here
		n, _ := openapi.IsNamespaceScoped(yaml.TypeMeta{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
		})

		ar := metav1.APIResource{
			Name:       buildPluralName(gvk.Kind),
			Namespaced: n,
			Group:      gvk.Group,
			Version:    gvk.Version,
			Kind:       gvk.Kind,
			Verbs:      []string{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
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

func buildPluralName(n string) string {
	return strings.ToLower(n) + "s"
}
