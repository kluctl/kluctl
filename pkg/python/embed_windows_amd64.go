package python

import "embed"

//go:embed embed/python-windows-amd64.*
var pythonLib embed.FS
