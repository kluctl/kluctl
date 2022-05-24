package commands

import (
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/seal"
	"io/fs"
	"path/filepath"
	"strings"
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
	err := filepath.WalkDir(cmd.renderDir, func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(p, deployment.SealmeExt) {
			return nil
		}

		relPath, err := filepath.Rel(cmd.renderDir, p)
		if err != nil {
			return err
		}
		relTargetFile := filepath.Join(filepath.Dir(relPath), cmd.outputPattern, filepath.Base(p))
		targetFile, err := securejoin.SecureJoin(cmd.sealedSecretsDir, relTargetFile)
		if err != nil {
			return err
		}
		targetFile = targetFile[:len(targetFile)-len(deployment.SealmeExt)]
		err = sealer.SealFile(p, targetFile)
		if err != nil {
			return fmt.Errorf("failed sealing %s: %w", filepath.Base(p), err)
		}
		return nil
	})
	return err
}
