package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-lib-windows.tar.gz ../../build-python/windows *

//go:embed python-lib-windows.tar.gz python-lib-windows.tar.gz.files
var pythonLib embed.FS
