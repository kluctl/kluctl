package python

import "embed"

//go:embed python-lib-windows.tar.gz
var pythonLib embed.FS
