package vars

import (
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func newJinja2Must(t *testing.T) *jinja2.Jinja2 {
	j2, err := kluctl_jinja2.NewKluctlJinja2(true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		j2.Close()
	})
	return j2
}

func TestVarsCtx(t *testing.T) {
	j2 := newJinja2Must(t)

	varsCtx := NewVarsCtx(j2)
	varsCtx.Update(uo.FromMap(map[string]interface{}{
		"test1": map[string]interface{}{
			"test2": 42,
		},
	}))
	v, _, _ := varsCtx.Vars.GetNestedInt("test1", "test2")
	assert.Equal(t, int64(42), v)
}

func TestVarsCtxChild(t *testing.T) {
	j2 := newJinja2Must(t)

	varsCtx := NewVarsCtx(j2)
	varsCtx.UpdateChild("child", uo.FromMap(map[string]interface{}{
		"test1": map[string]interface{}{
			"test2": 42,
		},
	}))
	v, _, _ := varsCtx.Vars.GetNestedInt("child", "test1", "test2")
	assert.Equal(t, int64(42), v)
}

func TestVarsCtxStruct(t *testing.T) {
	j2 := newJinja2Must(t)

	varsCtx := NewVarsCtx(j2)

	s := struct {
		Test1 struct {
			Test2 int `json:"test2"`
		} `json:"test1"`
	}{
		Test1: struct {
			Test2 int `json:"test2"`
		}{Test2: 42},
	}

	err := varsCtx.UpdateChildFromStruct("child", s)
	assert.NoError(t, err)

	v, _, _ := varsCtx.Vars.GetNestedInt("child", "test1", "test2")
	assert.Equal(t, int64(42), v)
}
