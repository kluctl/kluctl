package deployment

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/pkg/k8s"
	"github.com/kluctl/kluctl/pkg/types"
	k8s2 "github.com/kluctl/kluctl/pkg/types/k8s"
	"github.com/kluctl/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"path/filepath"
	"sync"
)

type DeploymentCollection struct {
	Project   *DeploymentProject
	Images    *Images
	Inclusion *utils.Inclusion
	RenderDir string
	forSeal   bool

	Deployments []*DeploymentItem
	mutex       sync.Mutex
}

func NewDeploymentCollection(project *DeploymentProject, images *Images, inclusion *utils.Inclusion, renderDir string, forSeal bool) (*DeploymentCollection, error) {
	dc := &DeploymentCollection{
		Project:   project,
		Images:    images,
		Inclusion: inclusion,
		RenderDir: renderDir,
		forSeal:   forSeal,
	}

	indexes := make(map[string]int)
	deployments, err := dc.collectDeployments(project, indexes)
	if err != nil {
		return nil, err
	}
	dc.Deployments = deployments
	return dc, nil
}

func (c *DeploymentCollection) createBarrierDummy(project *DeploymentProject) *DeploymentItem {
	b := true
	tmpDiConfig := &types.DeploymentItemConfig{
		Barrier: &b,
	}
	di, err := NewDeploymentItem(project, c, tmpDiConfig, nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	return di
}

func findDeploymentItemIndex(project *DeploymentProject, pth *string, indexes map[string]int) (int, *string) {
	if pth == nil {
		return 0, nil
	}
	var dir2 *string
	index := 0
	dir := filepath.Join(project.dir, *pth)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		// we pre-checked directories, so this should not happen
		log.Fatal(err)
	}

	if _, ok := indexes[absDir]; !ok {
		indexes[absDir] = 0
	}
	index, _ = indexes[absDir]
	indexes[absDir] = index + 1
	dir2 = &absDir
	return index, dir2
}

func (c *DeploymentCollection) collectDeployments(project *DeploymentProject, indexes map[string]int) ([]*DeploymentItem, error) {
	var ret []*DeploymentItem

	for i, diConfig := range project.Config.Deployments {
		if diConfig.Include != nil {
			includedProject, ok := project.includes[i]
			if !ok {
				log.Fatalf("Did not find find index %d in project.includes", i)
			}
			ret2, err := c.collectDeployments(includedProject, indexes)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ret2...)
			if diConfig.Barrier != nil && *diConfig.Barrier {
				ret = append(ret, c.createBarrierDummy(project))
			}
		} else {
			index, dir2 := findDeploymentItemIndex(project, diConfig.Path, indexes)
			di, err := NewDeploymentItem(project, c, diConfig, dir2, index)
			if err != nil {
				return nil, err
			}
			ret = append(ret, di)
		}
	}

	return ret, nil
}

func (c *DeploymentCollection) RenderDeployments(k *k8s.K8sCluster) error {
	log.Infof("Rendering templates and Helm charts")

	wp := utils.NewDebuggerAwareWorkerPool(16)
	defer wp.StopWait(false)

	for _, d := range c.Deployments {
		err := d.render(k, c.forSeal, wp)
		if err != nil {
			return err
		}
	}
	err := wp.StopWait(true)
	if err != nil {
		return err
	}

	for _, d := range c.Deployments {
		err := d.renderHelmCharts(k, wp)
		if err != nil {
			return err
		}
	}
	err = wp.StopWait(false)
	if err != nil {
		return err
	}

	return nil
}

func (c *DeploymentCollection) resolveSealedSecrets() error {
	if c.forSeal {
		return nil
	}

	for _, d := range c.Deployments {
		err := d.resolveSealedSecrets("")
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *DeploymentCollection) buildKustomizeObjects(k *k8s.K8sCluster) error {
	log.Infof("Building kustomize objects")

	var wg sync.WaitGroup
	var errs []error
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(16)

	for _, d_ := range c.Deployments {
		d := d_

		wg.Add(1)
		go func() {
			err := d.buildKustomize()
			if err != nil {
				mutex.Lock()
				errs = append(errs, fmt.Errorf("building kustomize objects for %s failed. %w", *d.dir, err))
				mutex.Unlock()
			} else {
				wg.Add(1)
				go func() {
					_ = sem.Acquire(context.Background(), 1)
					defer sem.Release(1)

					err := d.postprocessAndLoadObjects(k, c.Images)
					if err != nil {
						mutex.Lock()
						errs = append(errs, fmt.Errorf("postprocessing kustomize objects for %s failed: %w", *d.dir, err))
						mutex.Unlock()
					}
					wg.Done()
				}()
			}

			wg.Done()
		}()
	}
	wg.Wait()

	if len(errs) != 0 {
		return utils.NewErrorList(errs)
	}
	return nil
}

func (c *DeploymentCollection) LocalObjectsByRef() map[k8s2.ObjectRef]bool {
	ret := make(map[k8s2.ObjectRef]bool)
	for _, d := range c.Deployments {
		for _, o := range d.Objects {
			ret[o.GetK8sRef()] = true
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

func (c *DeploymentCollection) Prepare(k *k8s.K8sCluster) error {
	err := c.RenderDeployments(k)
	if err != nil {
		return err
	}
	err = c.resolveSealedSecrets()
	if err != nil {
		return err
	}
	err = c.buildKustomizeObjects(k)
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
