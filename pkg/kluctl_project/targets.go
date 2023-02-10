package kluctl_project

import (
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"sort"
)

type dynamicTargetInfo struct {
	baseTarget    *types.Target
	dir           string
	gitProject    *types.GitProject
	ref           *string
	refPattern    *string
	defaultBranch string
}

func (c *LoadedKluctlProject) loadTargets() error {
	status.Trace(c.ctx, "Loading targets")
	defer status.Trace(c.ctx, "Done loading targets")

	targetNames := make(map[string]bool)
	c.Targets = nil

	for _, configTarget := range c.Config.Targets {
		target, err := c.buildTarget(configTarget)
		if err != nil {
			status.Warning(c.ctx, "Failed to load dynamic target config for project: %v", err)
			continue
		}

		err = c.renderTarget(target)
		if err != nil {
			status.Warning(c.ctx, "Failed to load target %s: %v", target.Name, err)
			continue
		}

		if _, ok := targetNames[target.Name]; ok {
			status.Warning(c.ctx, "Duplicate target %s", target.Name)
		} else {
			targetNames[target.Name] = true
			c.Targets = append(c.Targets, target)
		}
	}
	sort.SliceStable(c.Targets, func(i, j int) bool {
		return c.Targets[i].Name < c.Targets[j].Name
	})
	return nil
}

func (c *LoadedKluctlProject) renderTarget(target *types.Target) error {
	// Try rendering the target multiple times, until all values can be rendered successfully. This allows the target
	// to reference itself in complex ways. We'll also try loading the cluster vars in each iteration.

	var retErr error
	for i := 0; i < 10; i++ {
		varsCtx, err := c.buildVars(target, false)
		if err != nil {
			return err
		}

		changed, err := varsCtx.RenderStruct(target)
		if err == nil && !changed {
			return nil
		}
		retErr = err
	}
	return retErr
}

func (c *LoadedKluctlProject) buildTarget(configTarget *types.Target) (*types.Target, error) {
	var target types.Target
	err := utils.DeepCopy(&target, configTarget)
	if err != nil {
		return nil, err
	}
	if target.Discriminator == "" {
		target.Discriminator = c.Config.Discriminator
	}
	return &target, nil
}
