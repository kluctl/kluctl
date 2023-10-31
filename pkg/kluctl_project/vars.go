package kluctl_project

import (
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
)

func (p *LoadedKluctlProject) BuildVars(target *types.Target, forSeal bool) (*vars.VarsCtx, error) {
	varsCtx := vars.NewVarsCtx(p.J2)

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return nil, err
	}
	varsCtx.UpdateChild("target", targetVars)

	allArgs := uo.New()

	if target != nil {
		if target.Args != nil {
			allArgs.Merge(target.Args)
		}
		if forSeal {
			if target.SealingConfig.Args != nil {
				allArgs.Merge(target.SealingConfig.Args)
			}
		}
	}
	if p.LoadArgs.ExternalArgs != nil {
		allArgs.Merge(p.LoadArgs.ExternalArgs)
	}

	err = deployment.LoadDefaultArgs(p.Config.Args, allArgs)
	if err != nil {
		return nil, err
	}

	varsCtx.UpdateChild("args", allArgs)

	return varsCtx, nil
}
