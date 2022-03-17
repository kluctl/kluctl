package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	goversion "github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	deprecatedResources = map[schema.GroupKind]bool{
		{Group: "extensions", Kind: "Ingress"}: true,
	}
)

type K8sCluster struct {
	context string
	DryRun  bool

	discovery  *disk.CachedDiscoveryClient
	clientPool chan *parallelClientEntry

	ServerVersion *goversion.Version

	allResources       map[schema.GroupVersionKind]*v1.APIResource
	preferredResources map[schema.GroupKind]*v1.APIResource
	crdCache           map[k8s.ObjectRef]struct {
		crd *uo.UnstructuredObject
		err error
	}
	mutex sync.Mutex
}

type parallelClientEntry struct {
	http           *http.Client
	corev1         *corev1.CoreV1Client
	dynamicClient  dynamic.Interface
	metadataClient metadata.Interface

	warnings []ApiWarning
}

type ApiWarning struct {
	Code  int
	Agent string
	Text  string
}

func (p *parallelClientEntry) HandleWarningHeader(code int, agent string, text string) {
	p.warnings = append(p.warnings, ApiWarning{
		Code:  code,
		Agent: agent,
		Text:  text,
	})
}

func NewK8sCluster(context string, dryRun bool) (*K8sCluster, error) {
	k := &K8sCluster{
		context: context,
		DryRun:  dryRun,
	}

	configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: context}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	config.QPS = 10
	config.Burst = 20

	apiHost, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	discoveryCacheDir := filepath.Join(utils.GetTmpBaseDir(), "kube-cache/discovery", apiHost.Hostname())
	httpCacheDir := filepath.Join(utils.GetTmpBaseDir(), "kube-cache/http", apiHost.Hostname())
	discovery2, err := disk.NewCachedDiscoveryClientForConfig(dynamic.ConfigFor(config), discoveryCacheDir, httpCacheDir, time.Hour*24)
	if err != nil {
		return nil, err
	}
	k.discovery = discovery2

	parallelClients := 16
	k.clientPool = make(chan *parallelClientEntry, parallelClients)
	for i := 0; i < parallelClients; i++ {
		p := &parallelClientEntry{}
		config.WarningHandler = p

		p.http, err = rest.HTTPClientFor(config)
		if err != nil {
			return nil, err
		}

		p.corev1, err = corev1.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		p.dynamicClient, err = dynamic.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		p.metadataClient, err = metadata.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		k.clientPool <- p
	}

	v, err := k.discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	v2, err := goversion.NewVersion(v.String())
	if err != nil {
		return nil, err
	}
	k.ServerVersion = v2

	err = k.updateResources(true)
	if err != nil {
		return nil, err
	}

	return k, nil
}

func (k *K8sCluster) ReadWrite() *K8sCluster {
	k2 := *k
	k2.DryRun = false
	return &k2
}

func (k *K8sCluster) Context() string {
	return k.context
}

func (k *K8sCluster) updateResources(doLock bool) error {
	if doLock {
		k.mutex.Lock()
		defer k.mutex.Unlock()
	}

	k.allResources = map[schema.GroupVersionKind]*v1.APIResource{}
	k.preferredResources = map[schema.GroupKind]*v1.APIResource{}
	k.crdCache = map[k8s.ObjectRef]struct {
		crd *uo.UnstructuredObject
		err error
	}{}

	_, arls, err := k.discovery.ServerGroupsAndResources()
	if err != nil {
		return err
	}
	for _, arl := range arls {
		for _, ar := range arl.APIResources {
			if strings.Index(ar.Name, "/") != -1 {
				// skip subresources
				continue
			}
			gv, err := schema.ParseGroupVersion(arl.GroupVersion)
			if err != nil {
				continue
			}

			ar := ar
			ar.Group = gv.Group
			ar.Version = gv.Version

			gvk := schema.GroupVersionKind{
				Group:   ar.Group,
				Version: ar.Version,
				Kind:    ar.Kind,
			}
			if _, ok := deprecatedResources[gvk.GroupKind()]; ok {
				continue
			}
			if _, ok := k.allResources[gvk]; ok {
				ok = false
			}
			k.allResources[gvk] = &ar
		}
	}

	arls, err = k.discovery.ServerPreferredResources()
	for _, arl := range arls {
		for _, ar := range arl.APIResources {
			if strings.Index(ar.Name, "/") != -1 {
				// skip subresources
				continue
			}
			gv, err := schema.ParseGroupVersion(arl.GroupVersion)
			if err != nil {
				continue
			}

			ar := ar
			ar.Group = gv.Group
			ar.Version = gv.Version

			gk := schema.GroupKind{
				Group: ar.Group,
				Kind:  ar.Kind,
			}
			if _, ok := deprecatedResources[gk]; ok {
				continue
			}
			k.preferredResources[gk] = &ar
		}
	}
	return nil
}

func (k *K8sCluster) RediscoverResources() error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.discovery.Invalidate()
	return k.updateResources(false)
}

func (k *K8sCluster) IsNamespaced(gvk schema.GroupVersionKind) bool {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	r, ok := k.allResources[gvk]
	if !ok {
		return false
	}
	return r.Namespaced
}

func (k *K8sCluster) ShouldRemoveNamespace(ref k8s.ObjectRef) bool {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	r, ok := k.allResources[ref.GVK]
	if !ok {
		// we don't know, so don't remove the NS
		return false
	}
	if ref.Namespace == "" {
		return false
	}
	if r.Namespaced {
		return false
	}
	return true
}

func (k *K8sCluster) RemoveNamespaceIfNeeded(o *uo.UnstructuredObject) {
	ref := o.GetK8sRef()
	if !k.ShouldRemoveNamespace(ref) {
		return
	}
	o.SetK8sNamespace("")
}

func (k *K8sCluster) RemoveNamespaceFromRefIfNeeded(ref k8s.ObjectRef) k8s.ObjectRef {
	if !k.ShouldRemoveNamespace(ref) {
		return ref
	}
	ref.Namespace = ""
	return ref
}

func (k *K8sCluster) GetAllGroupVersions() ([]string, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	m := make(map[string]bool)
	var l []string

	for gvk, _ := range k.allResources {
		gv := gvk.GroupVersion().String()
		if _, ok := m[gv]; !ok {
			m[gv] = true
			l = append(l, gv)
		}
	}
	return l, nil
}

func (k *K8sCluster) GetFilteredGKs(filters []string) []schema.GroupKind {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	m := make(map[schema.GroupKind]bool)
	var l []schema.GroupKind
	for gk, ar := range k.preferredResources {
		found := len(filters) == 0
		for _, f := range filters {
			if ar.Name == f || ar.Group == f || ar.Kind == f {
				found = true
				break
			}
		}
		if found {
			if _, ok := m[gk]; !ok {
				m[gk] = true
				l = append(l, gk)
			}
		}
	}
	return l
}

func (k *K8sCluster) GetGVKs(group *string, version *string, kind *string) []schema.GroupVersionKind {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var ret []schema.GroupVersionKind
	for gvk := range k.allResources {
		if group != nil && *group != gvk.Group {
			continue
		}
		if version != nil && *version != gvk.Version {
			continue
		}
		if kind != nil && *kind != gvk.Kind {
			continue
		}
		ret = append(ret, gvk)
	}
	return ret
}

func (k *K8sCluster) getGVRForGVK(gvk schema.GroupVersionKind) (*schema.GroupVersionResource, bool, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, ok := k.allResources[gvk]
	if !ok {
		return nil, false, &meta.NoKindMatchError{
			GroupKind:        gvk.GroupKind(),
			SearchedVersions: []string{gvk.Version},
		}
	}

	return &schema.GroupVersionResource{
		Group:    ar.Group,
		Version:  ar.Version,
		Resource: ar.Name,
	}, ar.Namespaced, nil
}

func (k *K8sCluster) GetApiResourceForGVK(gvk schema.GroupVersionKind) *v1.APIResource {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, _ := k.allResources[gvk]
	return ar
}

func (k *K8sCluster) GetCRDForGVK(gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	ar := k.GetApiResourceForGVK(gvk)
	if ar == nil {
		return nil, fmt.Errorf("APIResource for %s not found", gvk.String())
	}

	crdRef := k8s.ObjectRef{
		GVK:  schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
		Name: fmt.Sprintf("%s.%s", ar.Name, ar.Group),
	}

	k.mutex.Lock()
	x, ok := k.crdCache[crdRef]
	k.mutex.Unlock()
	if ok {
		return x.crd, x.err
	}

	crd, _, err := k.GetSingleObject(crdRef)

	k.mutex.Lock()
	x.crd = crd
	x.err = err
	k.crdCache[crdRef] = x
	k.mutex.Unlock()

	return crd, err
}

func (k *K8sCluster) GetSchemaForGVK(gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	crd, err := k.GetCRDForGVK(gvk)
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

	for _, version := range versions {
		name, _, _ := version.GetNestedString("name")
		if name != gvk.Version {
			continue
		}
		s, ok, err := version.GetNestedObject("schema", "openAPIV3Schema")
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

func (k *K8sCluster) WithCoreV1(cb func(client *corev1.CoreV1Client) error) error {
	p := <-k.clientPool
	defer func() { k.clientPool <- p }()
	return cb(p.corev1)
}

func (k *K8sCluster) withDynamicClientForGVK(gvk schema.GroupVersionKind, namespace string, cb func(r dynamic.ResourceInterface) error) ([]ApiWarning, error) {
	gvr, namespaced, err := k.getGVRForGVK(gvk)
	if err != nil {
		return nil, err
	}

	p := <-k.clientPool
	defer func() { k.clientPool <- p }()

	p.warnings = nil

	if namespaced && namespace != "" {
		err = cb(p.dynamicClient.Resource(*gvr).Namespace(namespace))
		return append([]ApiWarning(nil), p.warnings...), err
	} else {
		err = cb(p.dynamicClient.Resource(*gvr))
		return append([]ApiWarning(nil), p.warnings...), err
	}
}

func (k *K8sCluster) withMetadataClientForGVK(gvk schema.GroupVersionKind, namespace string, cb func(r metadata.ResourceInterface) error) ([]ApiWarning, error) {
	gvr, namespaced, err := k.getGVRForGVK(gvk)
	if err != nil {
		return nil, err
	}

	p := <-k.clientPool
	defer func() { k.clientPool <- p }()

	p.warnings = nil

	if namespaced && namespace != "" {
		err = cb(p.metadataClient.Resource(*gvr).Namespace(namespace))
		return append([]ApiWarning(nil), p.warnings...), err
	} else {
		err = cb(p.metadataClient.Resource(*gvr))
		return append([]ApiWarning(nil), p.warnings...), err
	}
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

	apiWarnings, err := k.withDynamicClientForGVK(gvk, namespace, func(r dynamic.ResourceInterface) error {
		o := v1.ListOptions{
			LabelSelector: k.buildLabelSelector(labels),
		}
		x, err := r.List(context.Background(), o)
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

func (k *K8sCluster) ListObjectsMetadata(gvk schema.GroupVersionKind, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, []ApiWarning, error) {
	var result []*uo.UnstructuredObject

	apiWarnings, err := k.withMetadataClientForGVK(gvk, namespace, func(r metadata.ResourceInterface) error {
		o := v1.ListOptions{
			LabelSelector: k.buildLabelSelector(labels),
		}
		x, err := r.List(context.Background(), o)
		if err != nil {
			return err
		}
		for _, o := range x.Items {
			b, err := json.Marshal(o)
			if err != nil {
				log.Panic(err)
			}
			u, err := uo.FromString(string(b))
			if err != nil {
				log.Panic(err)
			}
			u.SetK8sGVK(gvk)
			result = append(result, u)
		}
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) ListAllObjects(verbs []string, namespace string, labels map[string]string, onlyMetadata bool) ([]*uo.UnstructuredObject, map[schema.GroupVersionKind][]ApiWarning, error) {
	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

	var ret []*uo.UnstructuredObject
	retApiWarnings := make(map[schema.GroupVersionKind][]ApiWarning)
	var mutex sync.Mutex

	k.mutex.Lock()
	for _, ar := range k.preferredResources {
		foundVerb := false
		for _, v := range verbs {
			if utils.FindStrInSlice(ar.Verbs, v) != -1 {
				foundVerb = true
				break
			}
		}
		if !foundVerb {
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   ar.Group,
			Version: ar.Version,
			Kind:    ar.Kind,
		}
		wp.Submit(func() error {
			var l []*uo.UnstructuredObject
			var apiWarnings []ApiWarning
			var err error
			if onlyMetadata {
				l, apiWarnings, err = k.ListObjectsMetadata(gvk, namespace, labels)
			} else {
				l, apiWarnings, err = k.ListObjects(gvk, namespace, labels)
			}
			if err != nil {
				return err
			}
			mutex.Lock()
			defer mutex.Unlock()
			ret = append(ret, l...)
			if len(apiWarnings) != 0 {
				retApiWarnings[gvk] = apiWarnings
			}
			return nil
		})
	}
	// release it early and let the goroutines finish (deadlocking otherwise)
	k.mutex.Unlock()

	err := wp.StopWait(false)
	if err != nil {
		return nil, retApiWarnings, err
	}
	return ret, retApiWarnings, nil
}

func (k *K8sCluster) GetSingleObject(ref k8s.ObjectRef) (*uo.UnstructuredObject, []ApiWarning, error) {
	var result *uo.UnstructuredObject
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		o := v1.GetOptions{}
		x, err := r.Get(context.Background(), ref.Name, o)
		if err != nil {
			return err
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) GetObjectsByRefs(refs []k8s.ObjectRef) ([]*uo.UnstructuredObject, map[k8s.ObjectRef][]ApiWarning, error) {
	wp := utils.NewWorkerPoolWithErrors(32)
	defer wp.StopWait(false)

	var ret []*uo.UnstructuredObject
	retApiWarnings := make(map[k8s.ObjectRef][]ApiWarning)
	var mutex sync.Mutex

	for _, ref_ := range refs {
		ref := ref_
		wp.Submit(func() error {
			o, apiWarnings, err := k.GetSingleObject(ref)
			mutex.Lock()
			defer mutex.Unlock()
			if len(apiWarnings) != 0 {
				retApiWarnings[ref] = apiWarnings
			}
			if err != nil {
				if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
					return nil
				}
				return err
			}
			ret = append(ret, o)
			return nil
		})
	}
	err := wp.StopWait(false)
	if err != nil {
		return nil, retApiWarnings, err
	}

	return ret, retApiWarnings, nil
}

type DeleteOptions struct {
	ForceDryRun         bool
	NoWait              bool
	IgnoreNotFoundError bool
}

func (k *K8sCluster) DeleteSingleObject(ref k8s.ObjectRef, options DeleteOptions) ([]ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun

	pp := v1.DeletePropagationForeground
	o := v1.DeleteOptions{
		PropagationPolicy: &pp,
	}
	if dryRun {
		o.DryRun = []string{"All"}
	}

	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		err := r.Delete(context.Background(), ref.Name, o)
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

		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

var v1_21, _ = goversion.NewVersion("1.21")
var v1_1000, _ = goversion.NewVersion("1.1000")

func (k *K8sCluster) FixObjectForPatch(o *uo.UnstructuredObject) *uo.UnstructuredObject {
	// A bug in versions < 1.20 cause errors when applying resources that have some fields omitted which have
	// default values. We need to fix these resources.
	// UPDATE even though https://github.com/kubernetes-sigs/structured-merge-diff/issues/130 says it's fixed, the
	// issue is still present.
	needsDefaultsFix := k.ServerVersion.LessThan(v1_21) || true
	// TODO check when this is actually fixed (see https://github.com/kubernetes/kubernetes/issues/94275)
	needsTypeConversionFix := k.ServerVersion.LessThan(v1_1000)
	if !needsDefaultsFix && !needsTypeConversionFix {
		return o
	}

	o = o.Clone()

	fixPorts := func(p string) {
		if !needsDefaultsFix {
			return
		}

		ports, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfMaps(o)
		if !found {
			return
		}

		for _, port := range ports {
			if _, ok := port["protocol"]; !ok {
				port["protocol"] = "TCP"
			}
		}
	}

	fixStringType := func(p string, k string) {
		if !needsTypeConversionFix {
			return
		}
		d, found, _ := uo.NewMyJsonPathMust(p).GetFirstMap(o)
		if !found {
			return
		}
		v, ok := d[k]
		if !ok {
			return
		}
		_, ok = v.(string)
		if !ok {
			d[k] = fmt.Sprintf("%v", v)
		}
	}

	fixContainer := func(p string) {
		fixPorts(p + ".ports")
		fixStringType(p+"resources.limits", "cpu")
		fixStringType(p+"resources.requests", "cpu")
	}

	fixContainers := func(p string) {
		containers, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfMaps(o)
		if !found {
			return
		}
		for i, _ := range containers {
			fixContainer(fmt.Sprintf("%s[%d]", p, i))
		}
	}

	fixLimits := func(p string) {
		limits, found, _ := uo.NewMyJsonPathMust(p).GetFirstListOfMaps(o)
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

func (k *K8sCluster) PatchObject(o *uo.UnstructuredObject, options PatchOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun
	ref := o.GetK8sRef()
	log2 := log.WithField("ref", ref)

	data, err := yaml.WriteYamlBytes(o)
	if err != nil {
		return nil, nil, err
	}

	po := v1.PatchOptions{
		FieldManager: "kluctl",
	}
	if dryRun {
		po.DryRun = []string{"All"}
	}
	if options.ForceApply {
		po.Force = &options.ForceApply
	}

	log2.Debugf("patching")
	var result *uo.UnstructuredObject
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Patch(context.Background(), ref.Name, types.ApplyPatchType, data, po)
		if err != nil {
			return fmt.Errorf("failed to patch %s: %w", ref.String(), err)
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
}

type UpdateOptions struct {
	ForceDryRun bool
}

func (k *K8sCluster) UpdateObject(o *uo.UnstructuredObject, options UpdateOptions) (*uo.UnstructuredObject, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun
	ref := o.GetK8sRef()
	log2 := log.WithField("ref", ref)

	updateOpts := v1.UpdateOptions{
		FieldManager: "kluctl",
	}
	if dryRun {
		updateOpts.DryRun = []string{"All"}
	}

	log2.Debugf("updating")
	var result *uo.UnstructuredObject
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Update(context.Background(), o.ToUnstructured(), updateOpts)
		if err != nil {
			return err
		}
		result = uo.FromUnstructured(x)
		return nil
	})
	return result, apiWarnings, err
}
