package python

import "embed"

//go:generate go run ./download darwin amd64 ../../download-python/darwin/amd64
//go:generate go run ../utils/embed_util/packer python-darwin-amd64.tar.gz ../../download-python/darwin/amd64/python/install bin lib/*.dylib lib/python3.*

//go:embed python-darwin-amd64.tar.gz python-darwin-amd64.tar.gz.files
var pythonLib embed.FS
