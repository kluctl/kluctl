package kluctl_project

import (
	"context"
	"fmt"
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

	if args.FromArchive != "" {
		if args.ProjectUrl != nil || args.ProjectRef != "" || args.ProjectConfig != "" || args.LocalClusters != "" || args.LocalDeployment != "" || args.LocalSealedSecrets != "" {
			return nil, fmt.Errorf("--from-archive can not be combined with any other project related option")
		}
		err := p.loadFromArchive()
		if err != nil {
			return nil, err
		}
		return p, nil
	} else {
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
}
