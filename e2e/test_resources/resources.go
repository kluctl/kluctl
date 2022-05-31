package test_resources

import "embed"

//go:embed *.yaml
var Yamls embed.FS
