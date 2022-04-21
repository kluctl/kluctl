package python

import "embed"

//go:generate go run ./download darwin arm64 ../../download-python/darwin/arm64
//go:generate go run ../utils/embed_util/packer python-darwin-arm64.tar.gz ../../download-python/darwin/arm64/python/install bin lib/*.dylib lib/python3.*

//go:embed python-darwin-arm64.tar.gz python-darwin-arm64.tar.gz.files
var pythonLib embed.FS
