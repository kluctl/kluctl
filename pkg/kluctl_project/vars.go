package kluctl_project

import (
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
)

func (p *LoadedKluctlProject) BuildVars(target *types.Target) (*vars.VarsCtx, error) {
	varsCtx := vars.NewVarsCtx(p.J2)

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return nil, err
	}
	varsCtx.UpdateChild("target", targetVars)

	allArgs := uo.New()

	if target != nil && target.Args != nil {
		allArgs.Merge(target.Args)
	}
	if p.LoadArgs.ExternalArgs != nil {
		allArgs.Merge(p.LoadArgs.ExternalArgs)
	}

	err = LoadDefaultArgs(p.Config.Args, allArgs)
	if err != nil {
		return nil, err
	}

	varsCtx.UpdateChild("args", allArgs)

	return varsCtx, nil
}
