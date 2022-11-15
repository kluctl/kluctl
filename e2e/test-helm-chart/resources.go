package test_resources

import (
	"embed"
)

//go:embed all:*
var HelmChartFS embed.FS
