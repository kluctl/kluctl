package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/pkg/oci"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
)

type ociPullCmd struct {
	args.RegistryCredentials

	Url       string        `group:"misc" help:"Specifies the artifact URL." required:"true"`
	OutputDir string        `group:"misc" help:"Specifies the directory to extract the artifact into. It must be empty or not exist." required:"true"`
	Timeout   time.Duration `group:"misc" help:"Specifies the timeout for pulling the artifact." default:"10m"`
}

func (cmd *ociPullCmd) Help() string {
	return `The pull command downloads a project artifact from an OCI repository and
extracts it into the specified directory.`
}

func (cmd *ociPullCmd) Run(ctx context.Context) error {
	url, err := oci.ParseArtifactURL(cmd.Url)
	if err != nil {
		return err
	}
	if cmd.OutputDir == "" {
		return fmt.Errorf("--output-dir must not be empty")
	}
	outPath, err := filepath.Abs(cmd.OutputDir)
	if err != nil {
		return err
	}

	if cmd.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	client, err := newOCIClient(ctx, cmd.Url, &cmd.RegistryCredentials)
	if err != nil {
		return err
	}
	if err := prepareOCIPullDirectory(outPath); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	st := status.Startf(ctx, "Pulling artifact from %s", url)
	defer st.Failed()

	meta, err := client.Pull(ctx, url, outPath, oci.WithPullLayerType(oci.LayerTypeTarball))
	if err != nil {
		return fmt.Errorf("pulling artifact failed: %w", err)
	}

	st.UpdateAndInfoFallbackf("Artifact %s successfully pulled to %s", meta.Digest, outPath)
	st.Success()
	status.Flush(ctx)
	return nil
}

func prepareOCIPullDirectory(path string) error {
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0750)
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("%q is not empty", path)
	}
	return nil
}
