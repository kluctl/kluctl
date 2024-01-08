package kluctl_jinja2

import (
	"context"
	"github.com/kluctl/go-embed-python/embed_util"
	x "github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"path/filepath"
)

const parallelism = 4

func NewKluctlJinja2(ctx context.Context, strict bool) (*x.Jinja2, error) {
	tmpDir := filepath.Join(utils.GetCacheDir(ctx), "go-embed-jinja2")
	extSrc, err := embed_util.NewEmbeddedFilesWithTmpDir(ExtSource, filepath.Join(tmpDir, "kluctl-ext"), true)
	if err != nil {
		return nil, err
	}

	return x.NewJinja2("kluctl",
		parallelism,
		x.WithStrict(strict),
		x.WithExtension("jinja2.ext.loopcontrols"),
		x.WithExtension("go_jinja2.ext.kluctl"),
		x.WithExtension("go_jinja2.ext.time"),
		x.WithExtension("ext.images_ext.ImagesExtension"),
		x.WithPythonPath(extSrc.GetExtractedPath()),
		x.WithEmbeddedExtractDir(tmpDir),
	)
}
