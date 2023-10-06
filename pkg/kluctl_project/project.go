package kluctl_project

import (
	"fmt"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"time"
)

type LoadedKluctlProject struct {
	LoadArgs LoadKluctlProjectArgs
	LoadTime time.Time

	TmpDir string

	sealedSecretsDir string

	Config  types2.KluctlProject
	Targets []*types2.Target

	J2 *jinja2.Jinja2
	RP *repocache.GitRepoCache
}

func (c *LoadedKluctlProject) FindTarget(name string) (*types2.Target, error) {
	for _, target := range c.Targets {
		if target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}
