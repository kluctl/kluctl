package k8s

import (
	"context"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fake4 "k8s.io/client-go/discovery/fake"
	fake3 "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake2 "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

type fakeClientFactory struct {
	scheme           *runtime.Scheme
	mapper           meta.RESTMapper
	objects          []runtime.Object
	simpleClientSet  *fake.Clientset
	dynamicClientSet *fake3.FakeDynamicClient
	discovery        discovery.DiscoveryInterface

	errors []errorEntry
}

type errorEntry struct {
	gvr       schema.GroupVersionResource
	name      string
	namespace string
	retErr    error
}

func (f *fakeClientFactory) RESTConfig() *rest.Config {
	return nil
}

func (f *fakeClientFactory) GetCA() []byte {
	return []byte{}
}

func (f *fakeClientFactory) CloseIdleConnections() {
}

func (f *fakeClientFactory) Mapper() meta.RESTMapper {
	return f.mapper
}

func (f *fakeClientFactory) Client(wh rest.WarningHandler) (client.WithWatch, error) {
	return fake2.NewClientBuilder().
		WithScheme(f.scheme).
		WithRESTMapper(f.mapper).
		WithObjectTracker(f.dynamicClientSet.Tracker()).
		WithInterceptorFuncs(f.buildErrorInterceptor()).
		Build(), nil
}

func (f *fakeClientFactory) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	return f.discovery, nil
}

func (f *fakeClientFactory) CoreV1Client(wh rest.WarningHandler) (corev1.CoreV1Interface, error) {
	return f.simpleClientSet.CoreV1(), nil
}

func NewFakeClientFactory(objects ...runtime.Object) *fakeClientFactory {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	simpleClientSet := fake.NewSimpleClientset(objects...)
	dynamicClientSet := fake3.NewSimpleDynamicClient(scheme, objects...)
	dynamicClientSet.Fake.Resources = ConvertSchemeToAPIResources(scheme)
	dc := &fake4.FakeDiscovery{Fake: &dynamicClientSet.Fake}

	agrs, _ := restmapper.GetAPIGroupResources(dc)
	mapper := restmapper.NewDiscoveryRESTMapper(agrs)

	f := &fakeClientFactory{
		scheme:           scheme,
		mapper:           mapper,
		objects:          objects,
		simpleClientSet:  simpleClientSet,
		dynamicClientSet: dynamicClientSet,
		discovery:        dc,
	}
	simpleClientSet.PrependReactor("*", "*", f.errorReactor)
	dynamicClientSet.PrependReactor("*", "*", f.errorReactor)
	return f
}

type HasName interface {
	GetName() string
}

func (f *fakeClientFactory) findError(gvr schema.GroupVersionResource, name string, namespace string) error {
	for _, ee := range f.errors {
		if ee.gvr != gvr {
			continue
		}
		if ee.namespace != "" && ee.namespace != namespace {
			continue
		}

		if ee.name != "" && ee.namespace != name {
			continue
		}

		return ee.retErr
	}
	return nil
}

func (f *fakeClientFactory) buildErrorInterceptor() interceptor.Funcs {
	h := func(key *client.ObjectKey, obj runtime.Object) error {
		name := ""
		namespace := ""
		if key != nil {
			name = key.Name
			namespace = key.Namespace
		} else if o, ok := obj.(client.Object); ok {
			name = o.GetName()
			namespace = o.GetNamespace()
		}

		gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
		rm, err := f.mapper.RESTMapping(gk, obj.GetObjectKind().GroupVersionKind().Version)
		if err != nil {
			return nil
		}
		return f.findError(rm.Resource, name, namespace)
	}

	return interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			err := h(&key, obj)
			if err != nil {
				return err
			}
			return client.Get(ctx, key, obj)
		},
		List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			err := h(nil, list)
			if err != nil {
				return err
			}
			return client.List(ctx, list, opts...)
		},
	}
}

func (f *fakeClientFactory) errorReactor(action testing.Action) (handled bool, ret runtime.Object, err error) {
	a, ok := action.(HasName)
	if !ok {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, nil
	}

	err = f.findError(action.GetResource(), a.GetName(), action.GetNamespace())
	if err != nil {
		return true, nil, err
	}
	return false, nil, err
}

func (f *fakeClientFactory) AddError(gvr schema.GroupVersionResource, name string, namespace string, retErr error) {
	f.errors = append(f.errors, errorEntry{
		gvr:       gvr,
		name:      name,
		namespace: namespace,
		retErr:    retErr,
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
