package python

import "embed"

//go:generate bash ./tar-python.sh python-lib-windows.tar.gz windows '*'

//go:embed python-lib-windows.tar.gz
var pythonLib embed.FS
