package python

import "embed"

//go:embed python-lib-linux.tar.gz
var pythonLib embed.FS
