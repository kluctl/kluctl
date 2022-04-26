package python

import "embed"

//go:embed embed/python-darwin-amd64.*
var pythonLib embed.FS
