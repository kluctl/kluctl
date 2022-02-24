package python

import "embed"

//go:generate bash ./tar-python.sh python-lib-darwin.tar.gz darwin/cpython-install lib

//go:embed python-lib-darwin.tar.gz
var pythonLib embed.FS
