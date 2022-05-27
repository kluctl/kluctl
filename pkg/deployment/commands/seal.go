package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/seal"
	"path/filepath"
)

type SealCommand struct {
	c                *deployment.DeploymentCollection
	outputPattern    string
	renderDir        string
	sealedSecretsDir string
}

func NewSealCommand(c *deployment.DeploymentCollection, outputPattern string, renderDir string, sealedSecretsDir string) *SealCommand {
	return &SealCommand{
		c:                c,
		outputPattern:    outputPattern,
		renderDir:        renderDir,
		sealedSecretsDir: sealedSecretsDir,
	}
}

func (cmd *SealCommand) Run(sealer *seal.Sealer) error {
	for _, di := range cmd.c.Deployments {
		sealedSecrets, err := di.ListSealedSecrets("")
		if err != nil {
			return err
		}

		for _, relPath := range sealedSecrets {
			sealmeFile := filepath.Join(di.RenderedDir, relPath+deployment.SealmeExt)
			targetFile, err := di.BuildSealedSecretPath(relPath)
			if err != nil {
				return err
			}

			err = sealer.SealFile(sealmeFile, targetFile)
			if err != nil {
				return fmt.Errorf("failed sealing %s: %w", filepath.Base(relPath), err)
			}
		}
	}

	return nil
}
