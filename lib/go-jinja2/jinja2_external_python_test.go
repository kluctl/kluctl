package jinja2

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/kluctl/go-embed-python/python"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestExternalPython(t *testing.T) {
	rndName := fmt.Sprintf("test-%d", rand.Uint32())
	ep, err := python.NewEmbeddedPython(rndName)
	assert.NoError(t, err)

	j2 := newJinja2(t, WithPython(ep))
	t1 := newTemplateFile(t, `{{ "a" }}`)

	r1, err := j2.RenderFile(t1)
	assert.NoError(t, err)
	assert.Equal(t, "a", r1)
}

func TestSystemPython(t *testing.T) {
	ep := python.NewPython()
	if _, err := ep.GetExePath(); err != nil {
		t.Skipf("skipping due to missing python binary")
	}

	j2 := newJinja2(t, WithPython(ep))
	t1 := newTemplateFile(t, `{{ "a" }}`)

	r1, err := j2.RenderFile(t1)
	assert.NoError(t, err)
	assert.Equal(t, "a", r1)
}

func TestSystemPythonVersion(t *testing.T) {
	backup := minimumPythonVersion
	t.Cleanup(func() {
		minimumPythonVersion = backup
	})

	minimumPythonVersion = semver.MustParse("10.0.0")

	ep := python.NewPython()
	if _, err := ep.GetExePath(); err != nil {
		t.Skipf("skipping due to missing python binary")
	}

	_, err := newJinja2WithErr(t, WithPython(ep))
	assert.ErrorContains(t, err, "must be at least 10.0.0")
}
