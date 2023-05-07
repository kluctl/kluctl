package kluctl_project

import (
	"context"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/status"
)

func LoadKluctlProject(ctx context.Context, args LoadKluctlProjectArgs, tmpDir string, j2 *jinja2.Jinja2) (*LoadedKluctlProject, error) {
	status.Trace(ctx, "enter LoadKluctlProject")
	defer status.Trace(ctx, "leave LoadKluctlProject")

	p := &LoadedKluctlProject{
		ctx:           ctx,
		LoadArgs:      args,
		TmpDir:        tmpDir,
		J2:            j2,
		RP:            args.RP,
		SopsDecrypter: args.SopsDecrypter,
	}

	if p.SopsDecrypter == nil {
		p.SopsDecrypter = decryptor.NewDecryptor(args.ProjectDir, decryptor.MaxEncryptedFileSize)
		p.SopsDecrypter.AddLocalKeyService()
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
