package python

import "embed"

//go:embed embed/python-linux-amd64.*
var pythonLib embed.FS
