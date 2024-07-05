package k8s

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
)

type CrdCache struct {
	mutex  sync.Mutex
	crdMap map[string]*v1.CustomResourceDefinition
	gkMap  map[schema.GroupKind]*v1.CustomResourceDefinition
}

func (cc *CrdCache) UpdateForGroup(ctx context.Context, c client.Client, group string) error {
	crdGvk := schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	var l v12.PartialObjectMetadataList
	l.SetGroupVersionKind(crdGvk)

	err := c.List(ctx, &l)
	if err != nil {
		return err
	}

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	if cc.crdMap == nil {
		cc.crdMap = map[string]*v1.CustomResourceDefinition{}
		cc.gkMap = map[schema.GroupKind]*v1.CustomResourceDefinition{}
	}

	g := utils.NewGoHelperR[*v1.CustomResourceDefinition](ctx, 8)
	for _, crdm := range l.Items {
		s := strings.SplitN(crdm.Name, ".", 2)
		if s[1] != group {
			continue
		}

		oldCrdm, ok := cc.crdMap[crdm.Name]
		if !ok || oldCrdm.ResourceVersion != crdm.ResourceVersion {
			g.RunRE(func() (*v1.CustomResourceDefinition, error) {
				var crd v1.CustomResourceDefinition
				err := c.Get(ctx, client.ObjectKeyFromObject(&crdm), &crd)
				if err != nil {
					return nil, err
				}
				return &crd, nil
			})
		}
	}

	g.Wait()
	if err := g.ErrorOrNil(); err != nil {
		return err
	}

	for _, crd := range g.Results() {
		cc.crdMap[crd.Name] = crd

		gk := schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}
		cc.gkMap[gk] = crd
	}

	return nil
}

func (cc *CrdCache) GetCRDByGK(gk schema.GroupKind) *v1.CustomResourceDefinition {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	if cc.crdMap == nil {
		return nil
	}
	return cc.gkMap[gk]
}
