package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type K8sCluster struct {
	ctx context.Context

	DryRun bool

	clientFactory ClientFactory
	clients       *k8sClients

	ServerVersion *version.Info

	Resources *k8sResources
}

func NewK8sCluster(ctx context.Context, clientFactory ClientFactory, dryRun bool) (*K8sCluster, error) {
	var err error

	k := &K8sCluster{
		ctx:           ctx,
		DryRun:        dryRun,
		clientFactory: clientFactory,
	}

	k.Resources, err = newK8sResources(ctx, clientFactory)
	if err != nil {
		return nil, err
	}

	k.clients, err = newK8sClients(ctx, clientFactory, 16)
	if err != nil {
		return nil, err
	}

	v, err := k.Resources.discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	k.ServerVersion = v

	var wg sync.WaitGroup
	wg.Add(2)

	var err1 error
	var err2 error
	go func() {
		err1 = k.Resources.updateResources()
		wg.Done()
	}()
	go func() {
		err2 = k.Resources.updateResourcesFromCRDs(k.clients)
		wg.Done()
	}()
	wg.Wait()

	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	return k, nil
}

func (k *K8sCluster) ReadWrite() *K8sCluster {
	k2 := *k
	k2.DryRun = false
	return &k2
}

func (k *K8sCluster) GetCA() []byte {
	return k.clientFactory.GetCA()
}

func (k *K8sCluster) buildLabelSelector(labels map[string]string) string {
	ret := ""

	for k, v := range labels {
		if len(ret) != 0 {
			ret += ","
		}
		ret += fmt.Sprintf("%s=%s", k, v)
	}
	return ret
}

func (k *K8sCluster) ListObjects(gvk schema.GroupVersionKind, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, []ApiWarning, error) {
	var result []*uo.UnstructuredObject

	apiWarnings, err := k.clients.withDynamicClientForGVK(k.Resources, gvk, namespace, func(r dynamic.ResourceInterface) error {
		o := v1.ListOptions{
			LabelSelector: k.buildLabelSelector(labels),
		}
		x, err := r.List(k.ctx, o)
		if err != nil {
			return err
		}
		for _, o := range x.Items {
			result = append(result, uo.FromUnstructured(&o))
		}
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) GetSingleObject(ref k8s.ObjectRef) (*uo.UnstructuredObject, []ApiWarning, error) {
	var result *uo.UnstructuredObject
	apiWarnings, err := k.clients.withDynamicClientForGVK(k.Resources, ref.GroupVersionKind(), ref.Namespace, func(r dynamic.ResourceInterface) error {
		o := v1.GetOptions{}
		x, err := r.Get(k.ctx, ref.Name, o)
		if err != nil {
			return err
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
}

type DeleteOptions struct {
	ForceDryRun         bool
	NoWait              bool
	IgnoreNotFoundError bool
}

func (k *K8sCluster) DeleteSingleObject(ref k8s.ObjectRef, options DeleteOptions) ([]ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun

	pp := v1.DeletePropagationBackground
	o := v1.DeleteOptions{
		PropagationPolicy: &pp,
	}
	if dryRun {
		o.DryRun = []string{"All"}
	}

	apiWarnings, err := k.clients.withDynamicClientForGVK(k.Resources, ref.GroupVersionKind(), ref.Namespace, func(r dynamic.ResourceInterface) error {
		err := r.Delete(k.ctx, ref.Name, o)
		if err != nil {
			if options.IgnoreNotFoundError && errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
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

func (k *K8sCluster) doPatch(ref k8s.ObjectRef, data []byte, patchType types.PatchType, options PatchOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun

	po := v1.PatchOptions{
		FieldManager: "kluctl",
	}
	if dryRun {
		po.DryRun = []string{"All"}
	}
	if options.ForceApply {
		po.Force = &options.ForceApply
	}

	status.Trace(k.ctx, "patching %s", ref.String())

	var result *uo.UnstructuredObject
	apiWarnings, err := k.clients.withDynamicClientForGVK(k.Resources, ref.GroupVersionKind(), ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Patch(k.ctx, ref.Name, patchType, data, po)
		if err != nil {
			return fmt.Errorf("failed to patch %s: %w", ref.String(), err)
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) PatchObject(o *uo.UnstructuredObject, options PatchOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	data, err := yaml.WriteYamlBytes(o)
	if err != nil {
		return nil, nil, err
	}

	return k.doPatch(o.GetK8sRef(), data, types.ApplyPatchType, options)
}

type JsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func (k *K8sCluster) PatchObjectWithJsonPatch(ref k8s.ObjectRef, patch interface{}, options PatchOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	data, err := json.Marshal(patch)
	if err != nil {
		return nil, nil, err
	}
	return k.doPatch(ref, data, types.JSONPatchType, options)
}

type UpdateOptions struct {
	ForceDryRun bool
}

func (k *K8sCluster) UpdateObject(o *uo.UnstructuredObject, options UpdateOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun
	ref := o.GetK8sRef()

	updateOpts := v1.UpdateOptions{
		FieldManager: "kluctl",
	}
	if dryRun {
		updateOpts.DryRun = []string{"All"}
	}

	status.Trace(k.ctx, "updating %s", ref.String())

	var result *uo.UnstructuredObject
	apiWarnings, err := k.clients.withDynamicClientForGVK(k.Resources, ref.GroupVersionKind(), ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Update(k.ctx, o.ToUnstructured(), updateOpts)
		if err != nil {
			return err
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
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

func (k *K8sCluster) ToRESTConfig() (*rest.Config, error) {
	return k.clientFactory.RESTConfig(), nil
}

func (k *K8sCluster) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	d, err := k.clientFactory.DiscoveryClient()
	if err != nil {
		return nil, err
	}
	cd, ok := d.(discovery.CachedDiscoveryInterface)
	if !ok {
		return nil, fmt.Errorf("not a CachedDiscoveryInterface")
	}
	return cd, nil
}

func (k *K8sCluster) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}
