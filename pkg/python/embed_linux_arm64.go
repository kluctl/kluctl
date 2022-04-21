package python

import "embed"

//go:generate go run ./download linux arm64 ../../download-python/linux/arm64
//go:generate go run ../utils/embed_util/packer python-linux-arm64.tar.gz ../../download-python/linux/arm64/python/install bin lib/*.so* lib/python3.*

//go:embed python-linux-arm64.tar.gz python-linux-arm64.tar.gz.files
var pythonLib embed.FS
