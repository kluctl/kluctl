package kluctl_project

import (
	"context"
	"github.com/kluctl/kluctl/v2/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"sort"
)

func (c *LoadedKluctlProject) loadTargets(ctx context.Context) error {
	status.Trace(ctx, "Loading targets")
	defer status.Trace(ctx, "Done loading targets")

	targetNames := make(map[string]bool)
	c.Targets = nil

	if len(c.Config.Targets) == 0 {
		target, err := c.buildTarget(&types.Target{})
		if err != nil {
			return err
		}
		err = c.renderTarget(target)
		if err != nil {
			return err
		}
		c.NoNameTarget = target
		return nil
	}

	for i, configTarget := range c.Config.Targets {
		if configTarget.Name == "" {
			status.Errorf(ctx, "Target at index %d has no name", i)
			continue
		}

		target, err := c.buildTarget(&configTarget)
		if err != nil {
			status.Warningf(ctx, "Failed to load target config for project: %v", err)
			continue
		}

		err = c.renderTarget(target)
		if err != nil {
			status.Warningf(ctx, "Failed to load target %s: %v", target.Name, err)
			continue
		}

		if _, ok := targetNames[target.Name]; ok {
			status.Warningf(ctx, "Duplicate target %s", target.Name)
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
		varsCtx, err := c.BuildVars(target)
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
	target, err := utils.DeepClone(configTarget)
	if err != nil {
		return nil, err
	}
	if target.Discriminator == "" {
		target.Discriminator = c.Config.Discriminator
	}
	if target.Aws == nil {
		if c.Config.Aws != nil {
			target.Aws = c.Config.Aws
		} else {
			target.Aws = &types.AwsConfig{}
		}
	} else if c.Config.Aws != nil {
		if target.Aws.Profile == nil {
			target.Aws.Profile = c.Config.Aws.Profile
		}
		if target.Aws.ServiceAccount == nil {
			target.Aws.ServiceAccount = c.Config.Aws.ServiceAccount
		}
	}
	// just to make sure we don't later overwrite stuff from c.Config, which we might have copied into the target a few
	// lines above this
	target, err = utils.DeepClone(target)
	return target, nil
}
