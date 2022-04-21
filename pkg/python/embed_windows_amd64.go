package python

import "embed"

//go:generate go run ./download windows amd64 ../../download-python/windows/amd64
//go:generate go run ../utils/embed_util/packer python-windows-amd64.tar.gz ../../download-python/windows/amd64/python/install Lib DLLs *.dll *.exe

//go:embed python-windows-amd64.tar.gz python-windows-amd64.tar.gz.files
var pythonLib embed.FS
