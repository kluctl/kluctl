package python

import "embed"

//go:generate go run ./download linux amd64 ../../download-python/linux/amd64
//go:generate go run ../utils/embed_util/packer python-linux-amd64.tar.gz ../../download-python/linux/amd64/python/install bin lib/*.so* lib/python3.*

//go:embed python-linux-amd64.tar.gz python-linux-amd64.tar.gz.files
var pythonLib embed.FS
