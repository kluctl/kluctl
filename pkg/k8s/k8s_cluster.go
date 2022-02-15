package k8s

import (
	"context"
	"fmt"
	types2 "github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	goversion "github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"path"
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

	allResources       map[schema.GroupVersionKind]v1.APIResource
	preferredResources map[schema.GroupKind]v1.APIResource
	mutex              sync.Mutex
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

	home := homedir.HomeDir()
	if home == "" {
		return nil, fmt.Errorf("home dir could not be determined")
	}
	kubeconfig := path.Join(home, ".kube", "config")

	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: context}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	config.QPS = 10
	config.Burst = 20

	discovery2, err := disk.NewCachedDiscoveryClientForConfig(dynamic.ConfigFor(config), path.Join(utils.GetTmpBaseDir(), "kube-cache"), path.Join(utils.GetTmpBaseDir(), "kube-http-cache"), time.Minute*1)
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

	err = k.updateResources()
	if err != nil {
		return nil, err
	}

	return k, nil
}

func (k *K8sCluster) Context() string {
	return k.context
}

func (k *K8sCluster) updateResources() error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.allResources = map[schema.GroupVersionKind]v1.APIResource{}
	k.preferredResources = map[schema.GroupKind]v1.APIResource{}

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
			k.allResources[gvk] = ar
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
			k.preferredResources[gk] = ar
		}
	}
	return nil
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

func (k *K8sCluster) ShouldRemoveNamespace(ref types2.ObjectRef) bool {
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

func (k *K8sCluster) RemoveNamespaceIfNeeded(o *unstructured.Unstructured) {
	ref := types2.RefFromObject(o)
	if !k.ShouldRemoveNamespace(ref) {
		return
	}
	o.SetNamespace("")
}

func (k *K8sCluster) RemoveNamespaceFromRefIfNeeded(ref types2.ObjectRef) types2.ObjectRef {
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

func (k *K8sCluster) getGVRForGVK(gvk schema.GroupVersionKind) (*schema.GroupVersionResource, bool, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, ok := k.allResources[gvk]
	if !ok {
		return nil, false, errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, "gvk")
	}

	return &schema.GroupVersionResource{
		Group:    ar.Group,
		Version:  ar.Version,
		Resource: ar.Name,
	}, ar.Namespaced, nil
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

	if namespaced {
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

	if namespaced {
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

func (k *K8sCluster) ListObjects(gvk schema.GroupVersionKind, namespace string) (*unstructured.UnstructuredList, []ApiWarning, error) {
	var result *unstructured.UnstructuredList

	apiWarnings, err := k.withDynamicClientForGVK(gvk, namespace, func(r dynamic.ResourceInterface) error {
		o := v1.ListOptions{}
		x, err := r.List(context.Background(), o)
		if err != nil {
			return err
		}
		result = x
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) ListObjectsMetadata(gvk schema.GroupVersionKind, namespace string, labels map[string]string) (*v1.PartialObjectMetadataList, []ApiWarning, error) {
	var result *v1.PartialObjectMetadataList

	apiWarnings, err := k.withMetadataClientForGVK(gvk, namespace, func(r metadata.ResourceInterface) error {
		o := v1.ListOptions{
			LabelSelector: k.buildLabelSelector(labels),
		}
		x, err := r.List(context.Background(), o)
		if err != nil {
			return err
		}
		result = x
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) ListAllObjectsMetadata(verbs []string, namespace string, labels map[string]string) ([]*v1.PartialObjectMetadata, error) {
	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

	var ret []*v1.PartialObjectMetadata
	var retApiWarnings []ApiWarning
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
			lm, apiWarnings, err := k.ListObjectsMetadata(gvk, namespace, labels)
			if err != nil {
				return err
			}
			mutex.Lock()
			defer mutex.Unlock()
			for _, o := range lm.Items {
				o := o
				o.SetGroupVersionKind(gvk)
				ret = append(ret, &o)
			}
			retApiWarnings = append(retApiWarnings, apiWarnings...)
			return nil
		})
	}
	// release it early and let the goroutines finish (deadlocking otherwise)
	k.mutex.Unlock()

	err := wp.StopWait(false)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (k *K8sCluster) GetSingleObject(ref types2.ObjectRef) (*unstructured.Unstructured, []ApiWarning, error) {
	var result *unstructured.Unstructured
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		o := v1.GetOptions{}
		x, err := r.Get(context.Background(), ref.Name, o)
		if err != nil {
			return err
		}
		result = x
		return nil
	})
	return result, apiWarnings, err
}

func (k *K8sCluster) GetObjectsByRefs(refs []types2.ObjectRef) ([]*unstructured.Unstructured, map[types2.ObjectRef][]ApiWarning, error) {
	wp := utils.NewWorkerPoolWithErrors(32)
	defer wp.StopWait(false)

	var ret []*unstructured.Unstructured
	retApiWarnings := make(map[types2.ObjectRef][]ApiWarning)
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
				if errors.IsNotFound(err) {
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

func (k *K8sCluster) DeleteSingleObject(ref types2.ObjectRef, options DeleteOptions) ([]ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun

	pp := v1.DeletePropagationForeground
	o := v1.DeleteOptions{
		PropagationPolicy: &pp,
	}
	if dryRun {
		o.DryRun = []string{"All"}
	}

	return k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		err := r.Delete(context.Background(), ref.Name, o)
		if err != nil {
			if options.IgnoreNotFoundError && errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if !dryRun && !options.NoWait {
			err = k.waitForDeletedObject(r, ref)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (k *K8sCluster) waitForDeletedObject(r dynamic.ResourceInterface, ref types2.ObjectRef) error {
	for true {
		o := v1.GetOptions{}
		_, err := r.Get(context.Background(), ref.Name, o)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

var v1_21, _ = goversion.NewVersion("1.21")
var v1_1000, _ = goversion.NewVersion("1.1000")

func (k *K8sCluster) FixObjectForPatch(o *unstructured.Unstructured) *unstructured.Unstructured {
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

	o = uo.CopyUnstructured(o)

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

func (k *K8sCluster) PatchObject(o *unstructured.Unstructured, options PatchOptions) (*unstructured.Unstructured, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun
	ref := types2.RefFromObject(o)
	log2 := log.WithField("ref", ref)

	data, err := o.MarshalJSON()
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
	var result *unstructured.Unstructured
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Patch(context.Background(), ref.Name, types.ApplyPatchType, data, po)
		if err != nil {
			return fmt.Errorf("failed to patch %s: %w", ref.String(), err)
		}
		result = x
		return nil
	})
	return result, apiWarnings, err
}

type UpdateOptions struct {
	ForceDryRun bool
}

func (k *K8sCluster) UpdateObject(o *unstructured.Unstructured, options UpdateOptions) (*unstructured.Unstructured, []ApiWarning, error) {
	dryRun := k.DryRun || options.ForceDryRun
	ref := types2.RefFromObject(o)
	log2 := log.WithField("ref", ref)

	uo := v1.UpdateOptions{
		FieldManager: "kluctl",
	}
	if dryRun {
		uo.DryRun = []string{"All"}
	}

	log2.Debugf("updating")
	var result *unstructured.Unstructured
	apiWarnings, err := k.withDynamicClientForGVK(ref.GVK, ref.Namespace, func(r dynamic.ResourceInterface) error {
		x, err := r.Update(context.Background(), o, uo)
		if err != nil {
			return err
		}
		result = x
		return nil
	})
	return result, apiWarnings, err
}
