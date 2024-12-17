package kluctl_jinja2

import (
	"context"
	"github.com/kluctl/go-embed-python/embed_util"
	"github.com/kluctl/go-embed-python/python"
	x "github.com/kluctl/kluctl/lib/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"path/filepath"
	"testing"
)

const parallelism = 4

func NewKluctlJinja2(ctx context.Context, strict bool, useSystemPython bool) (*x.Jinja2, error) {
	tmpDir := filepath.Join(utils.GetCacheDir(ctx), "go-embed-jinja2")

	if testing.Testing() && os.Getenv("KLUCTL_TEST_JINJA2_CACHE") != "1" {
		// we share the cache for all tests, even though WithCacheDir is used in tests
		// otherwise things get really slow
		tmpDir = filepath.Join(os.TempDir(), "kluctl-tests-jinja2")
	}

	extSrc, err := embed_util.NewEmbeddedFilesWithTmpDir(ExtSource, filepath.Join(tmpDir, "kluctl-ext"), true)
	if err != nil {
		return nil, err
	}

	var systemPython python.Python
	if useSystemPython {
		systemPython = python.NewPython()
	}

	return x.NewJinja2("kluctl",
		parallelism,
		x.WithPython(systemPython),
		x.WithStrict(strict),
		x.WithExtension("jinja2.ext.loopcontrols"),
		x.WithExtension("go_jinja2.ext.kluctl"),
		x.WithExtension("go_jinja2.ext.time"),
		x.WithExtension("ext.images_ext.ImagesExtension"),
		x.WithPythonPath(extSrc.GetExtractedPath()),
		x.WithEmbeddedExtractDir(tmpDir),
	)
}
