package jinja2

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func newJinja2WithErr(t *testing.T, opts ...Jinja2Opt) (*Jinja2, error) {
	name := fmt.Sprintf("jinja2-%d", rand.Uint32())
	opts2 := append(opts, WithExtension("go_jinja2.ext.kluctl"))
	j2, err := NewJinja2(name, 1, opts2...)
	if err != nil {
		return nil, err
	}

	t.Cleanup(j2.Close)
	t.Cleanup(j2.Cleanup)

	return j2, nil
}

func newJinja2(t *testing.T, opts ...Jinja2Opt) *Jinja2 {
	j2, err := newJinja2WithErr(t, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return j2
}

func newTemplateFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-template-")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()
	_, err = tmpFile.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return tmpFile.Name()
}

func newTemplateDir(t *testing.T, contents map[string]string) string {
	dir := t.TempDir()
	for p, c := range contents {
		x := filepath.Join(dir, p)
		err := os.MkdirAll(filepath.Dir(x), 0o700)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(x, []byte(c), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func assertDirSame(t *testing.T, dir string, expected map[string]string) {
	found := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatal(err)
		}
		relPath = filepath.ToSlash(relPath)

		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		found[relPath] = string(b)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expected, found)
}

func TestJinja2(t *testing.T) {
	j2 := newJinja2(t, WithGlobals(map[string]any{
		"test_var1": "1",
		"test_var2": map[string]any{
			"test": "2",
		},
	}))

	s, err := j2.RenderString("test")
	assert.NoError(t, err)
	assert.Equal(t, "test", s)

	s, err = j2.RenderString("test - {{ test_var1 }}")
	assert.NoError(t, err)
	assert.Equal(t, "test - 1", s)

	s, err = j2.RenderString("test - {{ test_var2.test }}")
	assert.NoError(t, err)
	assert.Equal(t, "test - 2", s)

	s, err = j2.RenderString("test - {{ get_var('test_var2.test', 'd') }}", WithExtension("go_jinja2.ext.kluctl"))
	assert.NoError(t, err)
	assert.Equal(t, "test - 2", s)

	s, err = j2.RenderString("test - {{ get_var('test_var2.test1', 'd') }}", WithExtension("go_jinja2.ext.kluctl"))
	assert.NoError(t, err)
	assert.Equal(t, "test - d", s)

	s, err = j2.RenderString("test - {{ test_var1 | add(2) }}", WithFilter("add", `
def add(a, b):
	return int(a) + int(b)
`))
	assert.NoError(t, err)
	assert.Equal(t, "test - 3", s)

	s, err = j2.RenderString("test - {{ test_var2.test | mul(3) }}", WithFilter("mul:multiply", `
def multiply(a, b):
	return int(a) * int(b)
`))
	assert.NoError(t, err)
	assert.Equal(t, "test - 6", s)
}

type testStruct struct {
	V1 string         `json:"v1"`
	S1 testStruct2    `json:"s1"`
	M1 map[string]any `json:"m1"`
}

type testStruct2 struct {
	V2 string `json:"v2"`
}

func TestRenderStruct(t *testing.T) {
	j2 := newJinja2(t)

	s := testStruct{
		V1: `{{ "1" }}`,
		S1: testStruct2{
			V2: `{{ "2" }}`,
		},
		M1: map[string]any{
			"a": map[string]any{
				"b": `{{ "3" }}`,
			},
			`{{ "c" }}`: "4",
			`{{ "d" }}`: `{{ "5" }}`,
			`{{ "e" }}`: map[string]any{
				"f":         `{{ "6" }}`,
				`{{ "g" }}`: `{{ "7" }}`,
			},
		},
	}
	e := testStruct{
		V1: "1",
		S1: testStruct2{
			V2: "2",
		},
		M1: map[string]any{
			"a": map[string]any{
				"b": "3",
			},
			"c": "4",
			"d": "5",
			"e": map[string]any{
				"f": "6",
				"g": "7",
			},
		},
	}

	changed, err := j2.RenderStruct(&s)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, e, s)

	changed, err = j2.RenderStruct(&s)
	assert.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, e, s)
}

func TestRenderFiles(t *testing.T) {
	j2 := newJinja2(t)

	t1 := newTemplateFile(t, `{{ "a" }}`)
	t2 := newTemplateFile(t, `{{ "b" }}`)

	r1, err := j2.RenderFile(t1)
	assert.NoError(t, err)
	assert.Equal(t, "a", r1)

	r2, err := j2.RenderFile(t2)
	assert.NoError(t, err)
	assert.Equal(t, "b", r2)
}

func TestRenderRelativeFiles(t *testing.T) {
	j2 := newJinja2(t)

	d := newTemplateDir(t, map[string]string{
		"template":        "{{ a }}",
		"subdir/template": "{{ b }}",
	})

	r1, err := j2.RenderFile("template", WithSearchDirs([]string{d}), WithGlobal("a", "a"))
	assert.NoError(t, err)
	assert.Equal(t, "a", r1)

	r2, err := j2.RenderFile("template", WithSearchDirs([]string{filepath.Join(d, "subdir")}), WithGlobal("b", "b"))
	assert.NoError(t, err)
	assert.Equal(t, "b", r2)

	r3, err := j2.RenderFile("../template", WithSearchDirs([]string{filepath.Join(d, "subdir")}), WithGlobal("a", "b"))
	assert.NoError(t, err)
	assert.Equal(t, "b", r3)
}

func TestRenderFiles_Includes(t *testing.T) {
	includeDir := newTemplateDir(t, map[string]string{
		"include.yaml":        "test",
		"include-dot.yaml":    `{% include "./include.yaml" %}`,
		"include-caller.yaml": `{% include "include-caller2.yaml" %}`,
	})

	type testCase struct {
		name  string
		files map[string]string
		t     string   // templateNane
		sd    []string // searchDirs
		r     string   // result
		err   string
	}

	tests := []testCase{
		{
			name: "include with absolute file fails",
			files: map[string]string{
				"f.yaml": fmt.Sprintf(`{%% include "%s/include.yaml" %%}`, filepath.ToSlash(includeDir)),
			},
			t:   "f.yaml",
			err: fmt.Sprintf("template %s/include.yaml not found", filepath.ToSlash(includeDir)),
		},
		{
			name: "load_template with absolute file fails",
			files: map[string]string{
				"f.yaml": fmt.Sprintf(`{{ load_template("%s/include.yaml") }}`, filepath.ToSlash(includeDir)),
			},
			t:   "f.yaml",
			err: fmt.Sprintf("template %s/include.yaml not found", filepath.ToSlash(includeDir)),
		},
		{
			name: "load_base64 with absolute file fails",
			files: map[string]string{
				"f.yaml": fmt.Sprintf(`{{ load_base64("%s/include.yaml") }}`, filepath.ToSlash(includeDir)),
			},
			t:   "f.yaml",
			err: fmt.Sprintf("template %s/include.yaml not found", filepath.ToSlash(includeDir)),
		},
		{
			name: "include without searchdir fails",
			files: map[string]string{
				"f.yaml": `{% include "include.yaml" %}`,
			},
			t:   "f.yaml",
			err: "template include.yaml not found",
		},
		{
			name: "include with searchdir succeeds",
			files: map[string]string{
				"f.yaml": `{% include "include.yaml" %}`,
			},
			t:  "f.yaml",
			r:  "test",
			sd: []string{includeDir},
		},
		{
			name: "load_template without searchdir fails",
			files: map[string]string{
				"f.yaml": `{{ load_template("include.yaml") }}`,
			},
			t:   "f.yaml",
			err: "template include.yaml not found",
		},
		{
			name: "load_template with searchdir succeeds",
			files: map[string]string{
				"f.yaml": `{{ load_template("include.yaml") }}`,
			},
			t:  "f.yaml",
			r:  "test",
			sd: []string{includeDir},
		},
		{
			name: "load_base64 without searchdir fails",
			files: map[string]string{
				"f.yaml": `{{ load_base64("include.yaml") }}`,
			},
			t:   "f.yaml",
			err: "template include.yaml not found",
		},
		{
			name: "load_base64 with searchdir succeeds",
			files: map[string]string{
				"f.yaml": `{{ load_base64("include.yaml") }}`,
			},
			t:  "f.yaml",
			r:  "dGVzdA==",
			sd: []string{includeDir},
		},
		{
			name: "load_base64 with width argument",
			files: map[string]string{
				"f.yaml": `{{ load_base64("include.yaml", 2) }}`,
			},
			t:  "f.yaml",
			r:  "dG\nVz\ndA\n==",
			sd: []string{includeDir},
		},
		{
			name: "relative include fails without dot",
			files: map[string]string{
				"d1/include1.yaml": `{% include "include2.yaml" %}`,
				"d1/include2.yaml": "test2",
				"f.yaml":           `{% include "d1/include1.yaml" %}`,
			},
			t:   "f.yaml",
			sd:  []string{"self"},
			err: "template include2.yaml not found",
		},
		{
			name: "relative include succeeds with dot",
			files: map[string]string{
				"d1/include1.yaml": `{% include "./include2.yaml" %}`,
				"d1/include2.yaml": "test2",
				"f.yaml":           `{% include "d1/include1.yaml" %}`,
			},
			t:  "f.yaml",
			sd: []string{"self"},
			r:  "test2",
		},
		{
			name: "recursive include succeeds",
			files: map[string]string{
				"include2.yaml": `{% include "./include3.yaml" %}`,
				"include3.yaml": "test3",
				"f.yaml":        `{% include "./include2.yaml" %}`,
			},
			t:  "f.yaml",
			sd: []string{"self"},
			r:  "test3",
		},
		{
			name: "recursive include from searchdir succeeds",
			files: map[string]string{
				"f.yaml": `{% include "include-dot.yaml" %}`,
			},
			t:  "f.yaml",
			sd: []string{includeDir},
			r:  "test",
		},
		{
			name: "recursive include caller from searchdir succeeds",
			files: map[string]string{
				"include-caller2.yaml": "test-caller",
				"f.yaml":               `{% include "include-caller.yaml" %}`,
			},
			t:  "f.yaml",
			sd: []string{"self", includeDir},
			r:  "test-caller",
		},
	}

	j2 := newJinja2(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := newTemplateDir(t, tc.files)
			var sd []string
			for _, x := range tc.sd {
				if x == "self" {
					sd = append(sd, dir)
				} else {
					sd = append(sd, x)
				}
			}
			r, err := j2.RenderFile(filepath.Join(dir, tc.t), WithSearchDirs(sd))
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.err)
			}
			assert.Equal(t, tc.r, r)
		})
	}
}

func TestRenderDirectory(t *testing.T) {
	type testCase struct {
		name  string
		files map[string]string
		r     map[string]string
		sd    []string
		excl  []string
		err   string
	}

	includeDir := newTemplateDir(t, map[string]string{
		"include-dir.yaml": "test",
	})

	tests := []testCase{
		{
			name: "simple",
			files: map[string]string{
				"f1.yaml":    `{{ "a" }}`,
				"d1/f2.yaml": `{{ "b" }}`,
			},
			r: map[string]string{
				"f1.yaml":    `a`,
				"d1/f2.yaml": `b`,
			},
			sd: []string{"self"},
		},
		{
			name: "with include",
			files: map[string]string{
				"f1.yaml":          `{{ "a" }}`,
				"include.yaml":     `{% include "include2.yaml" %}`,
				"include2.yaml":    `{% include "d2/include3.yaml" %}`,
				"d1/f2.yaml":       `{% include "include.yaml" %}`,
				"d2/include3.yaml": `{{ "x" }}`,
			},
			r: map[string]string{
				"d1/f2.yaml":       "x",
				"d2/include3.yaml": "x",
				"f1.yaml":          "a",
				"include.yaml":     "x",
				"include2.yaml":    "x",
			},
			sd: []string{"self"},
		},
		{
			name: "with include and searchdir",
			files: map[string]string{
				"f1.yaml": `{% include "include-dir.yaml" %}`,
			},
			r: map[string]string{
				"f1.yaml": "test",
			},
			sd: []string{"self", includeDir},
		},
	}

	j2 := newJinja2(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := newTemplateDir(t, tc.files)
			var sd []string
			for _, x := range tc.sd {
				if x == "self" {
					sd = append(sd, dir)
				} else {
					sd = append(sd, x)
				}
			}
			targetDir := t.TempDir()
			err := j2.RenderDirectory(dir, targetDir, tc.excl, WithSearchDirs(sd))
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.err)
			}
			assertDirSame(t, targetDir, tc.r)
		})
	}
}

func TestCustomTmpDir(t *testing.T) {
	tmpDir := t.TempDir()

	// first, test without WithEmbeddedExtractDir
	j2 := newJinja2(t, WithGlobals(map[string]any{
		"test_var1": "1",
		"test_var2": map[string]any{
			"test": "2",
		},
	}))
	m, err := filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-python-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock
	m, err = filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-jinja2-lib-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock
	m, err = filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-jinja2-renderer-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock

	// and now with WithEmbeddedExtractDir
	j2 = newJinja2(t, WithGlobals(map[string]any{
		"test_var1": "1",
		"test_var2": map[string]any{
			"test": "2",
		},
	}), WithEmbeddedExtractDir(tmpDir))
	m, err = filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-python-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 0)
	m, err = filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-jinja2-lib-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 0)
	m, err = filepath.Glob(filepath.Join(os.TempDir(), "go-jinja2-embedded", j2.name+"-jinja2-renderer-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 0)
	m, err = filepath.Glob(filepath.Join(tmpDir, j2.name+"-python-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock
	m, err = filepath.Glob(filepath.Join(tmpDir, j2.name+"-jinja2-lib-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock
	m, err = filepath.Glob(filepath.Join(tmpDir, j2.name+"-jinja2-renderer-*"))
	assert.NoError(t, err)
	assert.Len(t, m, 2) // dir + .lock

	// now check if it is functional
	s, err := j2.RenderString("test")
	assert.NoError(t, err)
	assert.Equal(t, "test", s)
}
