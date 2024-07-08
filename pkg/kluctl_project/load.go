package kluctl_project

import (
	"context"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/lib/status"
	"time"
)

func LoadKluctlProject(ctx context.Context, args LoadKluctlProjectArgs, j2 *jinja2.Jinja2) (*LoadedKluctlProject, error) {
	status.Trace(ctx, "enter LoadKluctlProject")
	defer status.Trace(ctx, "leave LoadKluctlProject")

	p := &LoadedKluctlProject{
		LoadArgs: args,
		LoadTime: time.Now(),
		J2:       j2,
		GitRP:    args.GitRP,
		OciRP:    args.OciRP,
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
