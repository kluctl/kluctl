package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-lib-linux.tar.gz ../../build-python/linux/cpython-install lib

//go:embed python-lib-linux.tar.gz python-lib-linux.tar.gz.files
var pythonLib embed.FS
