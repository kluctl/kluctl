package k8s

import (
	"context"
	"fmt"
	"io"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

type K8sCluster struct {
	ctx context.Context

	DryRun bool

	clientFactory ClientFactory

	discovery      discovery.DiscoveryInterface
	discoveryMutex *sync.Mutex

	clients *k8sClients

	ServerVersion *version.Info
}

func NewK8sCluster(ctx context.Context, clientFactory ClientFactory, dryRun bool) (*K8sCluster, error) {
	var err error

	k := &K8sCluster{
		ctx:            ctx,
		DryRun:         dryRun,
		clientFactory:  clientFactory,
		discoveryMutex: &sync.Mutex{},
	}

	k.discovery, err = clientFactory.DiscoveryClient()
	if err != nil {
		return nil, err
	}

	k.clients, err = newK8sClients(ctx, clientFactory, 16)
	if err != nil {
		return nil, err
	}

	v, err := k.discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	k.ServerVersion = v

	return k, nil
}

func (k *K8sCluster) ReadWrite() *K8sCluster {
	k2 := *k
	k2.DryRun = false
	return &k2
}

func (k *K8sCluster) doList(l client.ObjectList, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, []ApiWarning, error) {
	apiWarnings, err := k.clients.withCClientFromPool(true, func(c client.Client) error {
		return c.List(k.ctx, l, client.InNamespace(namespace), client.MatchingLabels(labels))
	})
	if err != nil {
		return nil, apiWarnings, err
	}

	items, err := meta.ExtractList(l)
	if err != nil {
		return nil, apiWarnings, err
	}

	result := make([]*uo.UnstructuredObject, len(items))
	for i, o := range items {
		if u, ok := o.(*unstructured.Unstructured); ok {
			result[i] = uo.FromUnstructured(u)
		} else {
			x, err := uo.FromStruct(o)
			if err != nil {
				return nil, apiWarnings, err
			}
			result[i] = x
		}
	}

	return result, apiWarnings, err
}
func (k *K8sCluster) ListObjects(gvk schema.GroupVersionKind, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, []ApiWarning, error) {
	var l unstructured.UnstructuredList
	l.SetGroupVersionKind(gvk)
	return k.doList(&l, namespace, labels)
}

func (k *K8sCluster) ListMetadata(gvk schema.GroupVersionKind, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, []ApiWarning, error) {
	var l v1.PartialObjectMetadataList
	l.SetGroupVersionKind(gvk)
	return k.doList(&l, namespace, labels)
}

func (k *K8sCluster) GetSingleObject(ref k8s.ObjectRef) (*uo.UnstructuredObject, []ApiWarning, error) {
	var o unstructured.Unstructured
	o.SetGroupVersionKind(ref.GroupVersionKind())

	apiWarnings, err := k.clients.withCClientFromPool(true, func(c client.Client) error {
		return c.Get(k.ctx, client.ObjectKey{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		}, &o)

	})
	if err != nil {
		return nil, apiWarnings, err
	}
	return uo.FromUnstructured(&o), apiWarnings, nil
}

type DeleteOptions struct {
	ForceDryRun         bool
	NoWait              bool
	IgnoreNotFoundError bool
}

func (k *K8sCluster) DeleteSingleObject(ref k8s.ObjectRef, options DeleteOptions) ([]ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun

	var o unstructured.Unstructured
	o.SetGroupVersionKind(ref.GroupVersionKind())
	o.SetName(ref.Name)
	o.SetNamespace(ref.Namespace)

	apiWarnings, err := k.clients.withCClientFromPool(dryRun, func(c client.Client) error {
		return c.Delete(k.ctx, &o, client.PropagationPolicy(v1.DeletePropagationBackground))
	})

	if err != nil {
		if options.IgnoreNotFoundError && errors.IsNotFound(err) {
			return apiWarnings, nil
		}
		return apiWarnings, err
	}

	if !dryRun && !options.NoWait {
		err = k.waitForDeletedObject(ref)
		if err != nil {
			return apiWarnings, err
		}
	}

	return apiWarnings, nil
}

func (k *K8sCluster) waitForDeletedObject(ref k8s.ObjectRef) error {
	for true {
		_, _, err := k.GetSingleObject(ref)

		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		select {
		case <-time.After(500 * time.Millisecond):
			continue
		case <-k.ctx.Done():
			return fmt.Errorf("failed waiting for deletion of %s: %w", ref.String(), k.ctx.Err())
		}
	}
	return nil
}

func (k *K8sCluster) FixObjectForPatch(o *uo.UnstructuredObject) *uo.UnstructuredObject {
	// A bug in versions < 1.20 cause errors when applying resources that have some fields omitted which have
	// default values. We need to fix these resources.
	// UPDATE even though https://github.com/kubernetes-sigs/structured-merge-diff/issues/130 says it's fixed, the
	// issue is still present.
	k8sVersion, err := semver.NewVersion(k.ServerVersion.String())
	if err != nil {
		return o
	}
	needsDefaultsFix := k8sVersion.LessThan(semver.MustParse("1.21")) || true
	// TODO check when this is actually fixed (see https://github.com/kubernetes/kubernetes/issues/94275)
	needsTypeConversionFix := k8sVersion.LessThan(semver.MustParse("1.100"))
	if !needsDefaultsFix && !needsTypeConversionFix {
		return o
	}

	o = o.Clone()

	fixPorts := func(p string) {
		if !needsDefaultsFix {
			return
		}

		ports, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfObjects(o)
		if !found {
			return
		}

		for _, port := range ports {
			_, ok, _ := port.GetNestedField("protocol")
			if !ok {
				_ = port.SetNestedField("TCP", "protocol")
			}
		}
	}

	fixStringType := func(p string, k string) {
		if !needsTypeConversionFix {
			return
		}
		d, found, _ := uo.NewMyJsonPathMust(p).GetFirstObject(o)
		if !found {
			return
		}
		v, ok, _ := d.GetNestedField(k)
		if !ok {
			return
		}
		_, ok = v.(string)
		if !ok {
			_ = d.SetNestedField(fmt.Sprintf("%v", v), k)
		}
	}

	fixContainer := func(p string) {
		fixPorts(p + ".ports")
		fixStringType(p+".resources.limits", "cpu")
		fixStringType(p+".resources.requests", "cpu")
	}

	fixContainers := func(p string) {
		containers, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfObjects(o)
		if !found {
			return
		}
		for i, _ := range containers {
			fixContainer(fmt.Sprintf("%s[%d]", p, i))
		}
	}

	fixLimits := func(p string) {
		limits, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfObjects(o)
		if !found {
			return
		}
		for i, _ := range limits {
			fixStringType(fmt.Sprintf("%s[%d].default", p, i), "cpu")
			fixStringType(fmt.Sprintf("%s[%d].defaultRequest", p, i), "cpu")
		}
	}

	fixContainers("spec.template.spec.containers")
	fixPorts("spec.ports")
	fixLimits("spec.limits")

	return o
}

type PatchOptions struct {
	ForceDryRun bool
	ForceApply  bool
}

func (k *K8sCluster) doPatch(ref k8s.ObjectRef, obj client.Object, patch client.Patch, options PatchOptions) ([]ApiWarning, error) {
	status.Trace(k.ctx, "patching %s", ref.String())

	var opts []client.PatchOption
	if options.ForceApply {
		opts = append(opts, client.ForceOwnership)
	}
	if options.ForceDryRun {
		opts = append(opts, client.DryRunAll)
	}
	opts = append(opts, client.FieldOwner("kluctl"))

	apiWarnings, err := k.clients.withCClientFromPool(k.DryRun, func(c client.Client) error {
		err := c.Patch(k.ctx, obj, patch, opts...)
		if err != nil {
			return fmt.Errorf("failed to patch %s: %w", ref.String(), err)
		}
		return nil
	})
	return apiWarnings, err
}

func (k *K8sCluster) ApplyObject(o *uo.UnstructuredObject, options PatchOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	obj := o.Clone().ToUnstructured()
	apiWarnings, err := k.doPatch(o.GetK8sRef(), obj, client.Apply, options)
	if err != nil {
		return nil, apiWarnings, err
	}
	return uo.FromUnstructured(obj), apiWarnings, nil
}

type UpdateOptions struct {
	ForceDryRun bool
}

func (k *K8sCluster) UpdateObject(o *uo.UnstructuredObject, options UpdateOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	ref := o.GetK8sRef()
	obj := o.Clone().ToUnstructured()

	status.Trace(k.ctx, "updating %s", ref.String())

	var opts []client.UpdateOption
	if options.ForceDryRun {
		opts = append(opts, client.DryRunAll)
	}
	opts = append(opts, client.FieldOwner("kluctl"))

	apiWarnings, err := k.clients.withCClientFromPool(k.DryRun, func(c client.Client) error {
		return c.Update(k.ctx, obj, opts...)
	})
	if err != nil {
		return nil, apiWarnings, err
	}
	return uo.FromUnstructured(obj), apiWarnings, nil
}

// envtestProxyGet checks the environment variables KLUCTL_K8S_SERVICE_PROXY_XXX to enable testing of proxy requests with envtest
func (k *K8sCluster) envtestProxyGet(scheme, namespace, name, port, path string, params map[string]string) (io.ReadCloser, error) {
	for _, m := range utils.ParseEnvConfigSets("KLUCTL_K8S_SERVICE_PROXY") {
		envApiHost := strings.TrimSuffix(m["API_HOST"], "/")
		envNamespace := m["SERVICE_NAMESPACE"]
		envName := m["SERVICE_NAME"]
		envPort := m["SERVICE_PORT"]
		envUrl := m["LOCAL_URL"]

		apiHost := strings.TrimSuffix(k.clientFactory.RESTConfig().Host, "/")

		if envApiHost != apiHost || envNamespace != namespace || envName != name || port != envPort {
			continue
		}

		status.Trace(k.ctx, "envtestProxyGet apiHost=%s, ns=%s, name=%s, port=%s, path=%s, envUrl=%s", apiHost, namespace, name, port, path, envUrl)

		envUrl = fmt.Sprintf("%s%s", envUrl, path)
		resp, err := http.Get(envUrl)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("http get returned status %d: %s", resp.StatusCode, resp.Status)
		}
		return resp.Body, nil
	}
	return nil, nil
}

func (k *K8sCluster) ProxyGet(scheme, namespace, name, port, path string, params map[string]string) (io.ReadCloser, error) {
	stream, err := k.envtestProxyGet(scheme, namespace, name, port, path, params)
	if err != nil {
		return nil, err
	}
	if stream != nil {
		return stream, nil
	}

	var ret rest.ResponseWrapper
	_, err = k.clients.withClientFromPool(func(p *parallelClientEntry) error {
		ret = p.corev1.Services(namespace).ProxyGet(scheme, name, port, path, params)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret.Stream(k.ctx)
}

func (k *K8sCluster) IsNamespaced(gvk schema.GroupVersionKind) *bool {
	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(gvk)

	ret := false
	_, err := k.clients.withCClientFromPool(true, func(c client.Client) error {
		x, err := c.IsObjectNamespaced(&obj)
		if err != nil {
			return err
		}
		ret = x
		return nil
	})
	if err != nil {
		return nil
	}
	return &ret
}

func (k *K8sCluster) FixNamespace(o *uo.UnstructuredObject, def string) {
	ref := o.GetK8sRef()
	namespaced := k.IsNamespaced(ref.GroupVersionKind())
	if namespaced == nil {
		return
	}
	if !*namespaced && ref.Namespace != "" {
		o.SetK8sNamespace("")
	} else if *namespaced && ref.Namespace == "" {
		o.SetK8sNamespace(def)
	}
}

func (k *K8sCluster) FixNamespaceInRef(ref k8s.ObjectRef) k8s.ObjectRef {
	namespaced := k.IsNamespaced(ref.GroupVersionKind())
	if namespaced == nil {
		return ref
	}
	if !*namespaced && ref.Namespace != "" {
		ref.Namespace = ""
	} else if *namespaced && ref.Namespace == "" {
		ref.Namespace = "default"
	}
	return ref
}

func (k *K8sCluster) GetSchemaForGVK(gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	rms, err := k.clientFactory.Mapper().RESTMappings(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	var rm *meta.RESTMapping
	for _, x := range rms {
		if x.GroupVersionKind == gvk {
			rm = x
			break
		}
	}
	if rm == nil {
		return nil, fmt.Errorf("rest mapping not found")
	}

	ref := k8s.NewObjectRef(apiextensionsv1.GroupName, "v1", "CustomResourceDefinition", fmt.Sprintf("%s.%s", rm.Resource.Resource, gvk.Group), "")

	crd, _, err := k.GetSingleObject(ref)
	if err != nil {
		return nil, err
	}

	versions, ok, err := crd.GetNestedObjectList("spec", "versions")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("versions not found in CRD")
	}

	for _, v := range versions {
		name, _, _ := v.GetNestedString("name")
		if name != gvk.Version {
			continue
		}
		s, ok, err := v.GetNestedObject("schema", "openAPIV3Schema")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("version %s has no schema", name)
		}
		return s, nil
	}
	return nil, fmt.Errorf("schema for %s not found", gvk.String())
}

func (k *K8sCluster) ResetMapper() {
	if m, ok := k.clientFactory.Mapper().(meta.ResettableRESTMapper); ok {
		m.Reset()
	}
}

func (k *K8sCluster) ToClient() (client.WithWatch, error) {
	return k.clientFactory.Client(nil)
}

func (k *K8sCluster) ToRESTConfig() (*rest.Config, error) {
	return k.clientFactory.RESTConfig(), nil
}

func (k *K8sCluster) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	cd, ok := k.discovery.(discovery.CachedDiscoveryInterface)
	if !ok {
		return nil, fmt.Errorf("not a CachedDiscoveryInterface")
	}
	return cd, nil
}

func (k *K8sCluster) ToRESTMapper() (meta.RESTMapper, error) {
	return k.clientFactory.Mapper(), nil
}
