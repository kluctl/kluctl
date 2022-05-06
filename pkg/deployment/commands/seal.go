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
	c *deployment.DeploymentCollection
}

func NewSealCommand(c *deployment.DeploymentCollection) *SealCommand {
	return &SealCommand{
		c: c,
	}
}

func (cmd *SealCommand) Run(sealer *seal.Sealer) error {
	if cmd.c.Project.Config.SealedSecrets == nil || cmd.c.Project.Config.SealedSecrets.OutputPattern == nil {
		return fmt.Errorf("sealedSecrets.outputPattern is not defined")
	}

	err := filepath.WalkDir(cmd.c.RenderDir, func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(p, deployment.SealmeExt) {
			return nil
		}

		relPath, err := filepath.Rel(cmd.c.RenderDir, p)
		if err != nil {
			return err
		}
		relTargetFile := filepath.Join(filepath.Dir(relPath), *cmd.c.Project.Config.SealedSecrets.OutputPattern, filepath.Base(p))
		targetFile, err := securejoin.SecureJoin(cmd.c.Project.SealedSecretsDir, relTargetFile)
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
