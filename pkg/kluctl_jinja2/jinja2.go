package kluctl_jinja2

import (
	"github.com/kluctl/go-embed-python/embed_util"
	x "github.com/kluctl/go-jinja2"
)

const parallelism = 4

func NewKluctlJinja2(strict bool) (*x.Jinja2, error) {
	extSrc, err := embed_util.NewEmbeddedFiles(ExtSource, "kluctl-ext")
	if err != nil {
		return nil, err
	}

	return x.NewJinja2("kluctl",
		parallelism,
		x.WithStrict(strict),
		x.WithExtension("jinja2.ext.loopcontrols"),
		x.WithExtension("go_jinja2.ext.kluctl"),
		x.WithExtension("ext.images_ext.ImagesExtension"),
		x.WithPythonPath(extSrc.GetExtractedPath()))
}
