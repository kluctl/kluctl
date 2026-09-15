package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourcesIncludeRenderedChart(t *testing.T) {
	tests := []struct {
		name       string
		resources  []string
		outputPath string
		expected   bool
	}{
		{
			name:       "plain path",
			resources:  []string{"helm-rendered.yaml"},
			outputPath: "helm-rendered.yaml",
			expected:   true,
		},
		{
			name:       "dot slash prefixed path",
			resources:  []string{"./helm-rendered.yaml"},
			outputPath: "helm-rendered.yaml",
			expected:   true,
		},
		{
			name:       "dot slash prefixed output path",
			resources:  []string{"helm-rendered.yaml"},
			outputPath: "./helm-rendered.yaml",
			expected:   true,
		},
		{
			name:       "custom output path in a subdirectory",
			resources:  []string{"./rendered/chart.yaml"},
			outputPath: "rendered/chart.yaml",
			expected:   true,
		},
		{
			name:       "found among other resources",
			resources:  []string{"./configmap.yaml", "./helm-rendered.yaml"},
			outputPath: "helm-rendered.yaml",
			expected:   true,
		},
		{
			name:       "not included",
			resources:  []string{"./configmap.yaml"},
			outputPath: "helm-rendered.yaml",
			expected:   false,
		},
		{
			name:       "no resources at all",
			resources:  nil,
			outputPath: "helm-rendered.yaml",
			expected:   false,
		},
		{
			name:       "another chart's output path",
			resources:  []string{"./helm-rendered.yaml"},
			outputPath: "other-rendered.yaml",
			expected:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, resourcesIncludeRenderedChart(test.resources, test.outputPath))
		})
	}
}
