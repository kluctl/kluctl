package deployment

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"path/filepath"
	"sync"
)

type DeploymentCollection struct {
	ctx SharedContext

	Project   *DeploymentProject
	Images    *Images
	Inclusion *utils.Inclusion
	forSeal   bool

	Deployments []*DeploymentItem
	mutex       sync.Mutex
}

func NewDeploymentCollection(ctx SharedContext, project *DeploymentProject, images *Images, inclusion *utils.Inclusion, forSeal bool) (*DeploymentCollection, error) {
	dc := &DeploymentCollection{
		ctx:       ctx,
		Project:   project,
		Images:    images,
		Inclusion: inclusion,
		forSeal:   forSeal,
	}

	indexes := make(map[string]int)
	deployments, err := dc.collectAllDeployments(project, indexes)
	if err != nil {
		return nil, err
	}
	dc.Deployments = make([]*DeploymentItem, 0, len(deployments))
	for _, d := range deployments {
		if d.CheckInclusionForDeploy() {
			dc.Deployments = append(dc.Deployments, d)
		}
	}
	return dc, nil
}

func (c *DeploymentCollection) createBarrierDummy(project *DeploymentProject) *DeploymentItem {
	tmpDiConfig := &types.DeploymentItemConfig{
		Barrier: true,
	}
	di, err := NewDeploymentItem(c.ctx, project, c, tmpDiConfig, nil, 0)
	if err != nil {
		panic(err)
	}
	return di
}

func findDeploymentItemIndex(project *DeploymentProject, pth *string, indexes map[string]int) (int, *string) {
	if pth == nil {
		return 0, nil
	}
	var dir2 *string
	index := 0
	dir := filepath.Join(project.absDir, *pth)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		// we pre-checked directories, so this should not happen
		panic(err)
	}

	if _, ok := indexes[absDir]; !ok {
		indexes[absDir] = 0
	}
	index, _ = indexes[absDir]
	indexes[absDir] = index + 1
	dir2 = &absDir
	return index, dir2
}

func (c *DeploymentCollection) collectAllDeployments(project *DeploymentProject, indexes map[string]int) ([]*DeploymentItem, error) {
	var ret []*DeploymentItem

	if !kluctl_jinja2.IsConditionalTrue(project.Config.When) {
		return nil, nil
	}

	for i, diConfig := range project.Config.Deployments {
		if !kluctl_jinja2.IsConditionalTrue(diConfig.When) {
			continue
		}

		if diConfig.Include != nil || diConfig.Git != nil {
			includedProject, ok := project.includes[i]
			if !ok {
				panic(fmt.Sprintf("Did not find find index %d in project.includes", i))
			}
			ret2, err := c.collectAllDeployments(includedProject, indexes)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ret2...)
			if diConfig.Barrier {
				ret = append(ret, c.createBarrierDummy(project))
			}
		} else {
			index, dir2 := findDeploymentItemIndex(project, diConfig.Path, indexes)
			di, err := NewDeploymentItem(c.ctx, project, c, diConfig, dir2, index)
			if err != nil {
				return nil, err
			}
			ret = append(ret, di)
		}
	}

	return ret, nil
}

func (c *DeploymentCollection) RenderDeployments() error {
	s := status.Start(c.ctx.Ctx, "Rendering templates")
	defer s.Failed()

	g := utils.NewGoHelper(c.ctx.Ctx, 0)

	for _, d := range c.Deployments {
		d := d
		g.RunE(func() error {
			return d.render(c.forSeal)
		})
	}
	g.Wait()
	if g.ErrorOrNil() == nil {
		s.Success()
	}
	return g.ErrorOrNil()
}

func (c *DeploymentCollection) renderHelmCharts() error {
	s := status.Start(c.ctx.Ctx, "Rendering Helm Charts")
	defer s.Failed()

	g := utils.NewGoHelper(c.ctx.Ctx, 8)

	for _, d := range c.Deployments {
		d := d
		g.RunE(func() error {
			return d.renderHelmCharts()
		})
	}
	g.Wait()
	if g.ErrorOrNil() == nil {
		s.Success()
	}
	return g.ErrorOrNil()
}

func (c *DeploymentCollection) resolveSealedSecrets() error {
	if c.forSeal {
		return nil
	}

	for _, d := range c.Deployments {
		err := d.resolveSealedSecrets()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *DeploymentCollection) buildKustomizeObjects() error {
	g := utils.NewGoHelper(c.ctx.Ctx, 0)

	s := status.Start(c.ctx.Ctx, "Building kustomize objects")
	for _, d_ := range c.Deployments {
		d := d_
		g.RunE(func() error {
			err := d.buildKustomize()
			if err != nil {
				return fmt.Errorf("building kustomize objects for %s failed. %w", *d.dir, err)
			}
			return nil
		})
	}
	g.Wait()

	if g.ErrorOrNil() != nil {
		s.Failed()
		return g.ErrorOrNil()
	}
	s.Success()

	s = status.Start(c.ctx.Ctx, "Postprocessing objects")
	for _, d_ := range c.Deployments {
		d := d_
		g.RunE(func() error {
			err := d.postprocessCRDs()
			if err != nil {
				return fmt.Errorf("postprocessing CRDs failed: %w", err)
			}
			return nil
		})
	}
	g.Wait()

	g = utils.NewGoHelper(c.ctx.Ctx, 16)
	for _, d_ := range c.Deployments {
		d := d_

		g.RunE(func() error {
			err := d.postprocessObjects(c.Images)
			if err != nil {
				return fmt.Errorf("postprocessing kustomize objects for %s failed: %w", *d.dir, err)
			}
			return nil
		})
	}
	g.Wait()

	if g.ErrorOrNil() == nil {
		s.Success()
	} else {
		s.Failed()
	}

	return g.ErrorOrNil()
}

func (c *DeploymentCollection) LocalObjects() []*uo.UnstructuredObject {
	var ret []*uo.UnstructuredObject
	for _, d := range c.Deployments {
		ret = append(ret, d.Objects...)
	}
	return ret
}

func (c *DeploymentCollection) LocalObjectsByRef() map[k8s2.ObjectRef]*uo.UnstructuredObject {
	ret := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range c.Deployments {
		for _, o := range d.Objects {
			ret[o.GetK8sRef()] = o
		}
	}
	return ret
}

func (c *DeploymentCollection) LocalObjectRefs() []k8s2.ObjectRef {
	var ret []k8s2.ObjectRef
	for ref := range c.LocalObjectsByRef() {
		ret = append(ret, ref)
	}
	return ret
}

func (c *DeploymentCollection) Prepare() error {
	err := c.RenderDeployments()
	if err != nil {
		return err
	}
	err = c.renderHelmCharts()
	if err != nil {
		return err
	}
	err = c.resolveSealedSecrets()
	if err != nil {
		return err
	}
	err = c.buildKustomizeObjects()
	if err != nil {
		return err
	}
	return nil
}

func (c *DeploymentCollection) FindRenderedImages() map[k8s2.ObjectRef][]string {
	ret := make(map[k8s2.ObjectRef][]string)
	for _, d := range c.Deployments {
		for _, o := range d.Objects {
			ref := o.GetK8sRef()
			l, ok, _ := o.GetNestedObjectList("spec", "template", "spec", "containers")
			if !ok {
				continue
			}
			for _, c := range l {
				image, ok, _ := c.GetNestedString("image")
				if !ok {
					continue
				}
				ret[ref] = append(ret[ref], image)
			}
		}
	}
	return ret
}
