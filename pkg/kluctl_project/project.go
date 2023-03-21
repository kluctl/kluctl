package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
)

type LoadedKluctlProject struct {
	ctx context.Context

	LoadArgs LoadKluctlProjectArgs

	TmpDir string

	projectRootDir string
	ProjectDir     string

	sealedSecretsDir string

	Config  types2.KluctlProject
	Targets []*types2.Target

	J2            *jinja2.Jinja2
	RP            *repocache.GitRepoCache
	SopsDecrypter *decryptor.Decryptor
}

func (c *LoadedKluctlProject) FindTarget(name string) (*types2.Target, error) {
	for _, target := range c.Targets {
		if target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}
