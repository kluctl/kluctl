package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-darwin.tar.gz ../../download-python/darwin/python/install bin lib/*.dylib lib/python3.*

//go:embed python-darwin.tar.gz python-darwin.tar.gz.files
var pythonLib embed.FS
