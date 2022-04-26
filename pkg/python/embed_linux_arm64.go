package python

import "embed"

//go:embed embed/python-linux-arm64.*
var pythonLib embed.FS
