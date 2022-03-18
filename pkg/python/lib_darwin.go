package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-lib-darwin.tar.gz ../../build-python/darwin/cpython-install lib

//go:embed python-lib-darwin.tar.gz python-lib-darwin.tar.gz.files
var pythonLib embed.FS
