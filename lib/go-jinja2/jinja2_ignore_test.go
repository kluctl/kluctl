package jinja2

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestRenderDirectoryTemplateIgnore(t *testing.T) {
	type testCase struct {
		name  string
		files map[string]string
		r     map[string]string

		subdir     string
		ignoreRoot string
	}

	tests := []testCase{
		{
			name: "ignore /f1.yaml",
			files: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `{{ "a" }}`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": `/f1.yaml`,
			},
			r: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `a`,
				"d1/f1.yaml":      `a`,
				".templateignore": `/f1.yaml`,
			},
		},
		{
			name: "ignore /d1/f1.yaml",
			files: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `{{ "a" }}`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": `/d1/f1.yaml`,
			},
			r: map[string]string{
				"f1.yaml":         `a`,
				"f2.yaml":         `a`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": `/d1/f1.yaml`,
			},
		},
		{
			name: "ignore all f1.yaml",
			files: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `{{ "a" }}`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": `f1.yaml`,
			},
			r: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `a`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": `f1.yaml`,
			},
		},
		{
			name: "ignore f1.yaml via subdir-templateincode",
			files: map[string]string{
				"f1.yaml":            `{{ "a" }}`,
				"f2.yaml":            `{{ "a" }}`,
				"d1/f1.yaml":         `{{ "a" }}`,
				"d1/.templateignore": `f1.yaml`,
			},
			r: map[string]string{
				"f1.yaml":            `a`,
				"f2.yaml":            `a`,
				"d1/f1.yaml":         `{{ "a" }}`,
				"d1/.templateignore": `f1.yaml`,
			},
		},
		{
			name: "no ignore via root templateignore",
			files: map[string]string{
				"d1/f1.yaml":      `{{ "a" }}`,
				"d1/f2.yaml":      `{{ "a" }}`,
				"d1/d2/f1.yaml":   `{{ "a" }}`,
				".templateignore": `f1.yaml`,
			},
			r: map[string]string{
				"f1.yaml":    `a`,
				"f2.yaml":    `a`,
				"d2/f1.yaml": `a`,
			},
			subdir: "d1",
		},
		{
			name: "ignore via root templateignore",
			files: map[string]string{
				"d1/f1.yaml":            `{{ "a" }}`,
				"d1/f2.yaml":            `{{ "a" }}`,
				"d1/f3.yaml":            `{{ "a" }}`,
				"d1/d2/f1.yaml":         `{{ "a" }}`,
				"d1/d2/f3.yaml":         `{{ "a" }}`,
				".templateignore":       `f1.yaml`,
				"d1/d2/.templateignore": `f3.yaml`,
			},
			r: map[string]string{
				"f1.yaml":            `{{ "a" }}`,
				"f2.yaml":            `a`,
				"f3.yaml":            `a`,
				"d2/f1.yaml":         `{{ "a" }}`,
				"d2/f3.yaml":         `{{ "a" }}`,
				"d2/.templateignore": `f3.yaml`,
			},
			subdir:     "d1",
			ignoreRoot: ".",
		},
		{
			name: "reinclude /d1/f1.yaml",
			files: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `{{ "a" }}`,
				"d1/f1.yaml":      `{{ "a" }}`,
				".templateignore": "f1.yaml\n!d1/f1.yaml",
			},
			r: map[string]string{
				"f1.yaml":         `{{ "a" }}`,
				"f2.yaml":         `a`,
				"d1/f1.yaml":      `a`,
				".templateignore": "f1.yaml\n!d1/f1.yaml",
			},
		},
		{
			name: "complex reinclude",
			files: map[string]string{
				"d1/some/renderThese/f1.yaml": `{{ "a" }}`,
				"d1/some/renderThese/f2.txt":  `{{ "a" }}`,
				"d1/some/f2.yaml":             `{{ "a" }}`,
				".templateignore":             "**/some/**/*.*\n!**/some/renderThese/*.y*ml",
			},
			r: map[string]string{
				"d1/some/renderThese/f1.yaml": `a`,
				"d1/some/renderThese/f2.txt":  `{{ "a" }}`,
				"d1/some/f2.yaml":             `{{ "a" }}`,
				".templateignore":             "**/some/**/*.*\n!**/some/renderThese/*.y*ml",
			},
		},
	}

	j2 := newJinja2(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := newTemplateDir(t, tc.files)
			targetDir := t.TempDir()
			var opts []Jinja2Opt
			if tc.ignoreRoot != "" {
				opts = append(opts, WithTemplateIgnoreRootDir(filepath.Join(dir, tc.ignoreRoot)))
			}
			err := j2.RenderDirectory(filepath.Join(dir, tc.subdir), targetDir, nil, opts...)
			assert.NoError(t, err)
			assertDirSame(t, targetDir, tc.r)
		})
	}
}
