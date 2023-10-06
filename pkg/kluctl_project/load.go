package kluctl_project

import (
	"context"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"time"
)

func LoadKluctlProject(ctx context.Context, args LoadKluctlProjectArgs, tmpDir string, j2 *jinja2.Jinja2) (*LoadedKluctlProject, error) {
	status.Trace(ctx, "enter LoadKluctlProject")
	defer status.Trace(ctx, "leave LoadKluctlProject")

	p := &LoadedKluctlProject{
		LoadArgs: args,
		LoadTime: time.Now(),
		TmpDir:   tmpDir,
		J2:       j2,
		RP:       args.RP,
	}

	err := p.loadKluctlProject(ctx)
	if err != nil {
		return nil, err
	}
	err = p.loadTargets(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}
