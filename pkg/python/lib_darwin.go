package python

import "embed"

//go:embed python-lib-darwin.tar.gz
var pythonLib embed.FS
