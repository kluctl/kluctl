package python

import "embed"

//go:generate bash ./tar-python.sh python-lib-linux.tar.gz linux/cpython-install lib

//go:embed python-lib-linux.tar.gz
var pythonLib embed.FS
