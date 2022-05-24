package kluctl_project

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
)

func LoadKluctlProject(ctx context.Context, args LoadKluctlProjectArgs, tmpDir string, j2 *jinja2.Jinja2) (*LoadedKluctlProject, error) {

	p := &LoadedKluctlProject{
		ctx:      ctx,
		loadArgs: args,
		TmpDir:   tmpDir,
		J2:       j2,
		RP:       args.RP,
	}

	err := p.loadKluctlProject()
	if err != nil {
		return nil, err
	}
	err = p.loadTargets()
	if err != nil {
		return nil, err
	}
	return p, nil
}
