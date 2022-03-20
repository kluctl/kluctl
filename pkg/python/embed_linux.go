package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-linux.tar.gz ../../download-python/linux/python/install bin lib/*.so* lib/python3.*

//go:embed python-linux.tar.gz python-linux.tar.gz.files
var pythonLib embed.FS
