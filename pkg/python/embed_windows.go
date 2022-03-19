package python

import "embed"

//go:generate go run ../utils/embed_util/packer python-windows.tar.gz ../../download-python/windows/python/install Lib DLLs *.dll *.exe

//go:embed python-windows.tar.gz python-windows.tar.gz.files
var pythonLib embed.FS
