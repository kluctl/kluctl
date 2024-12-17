package jinja2

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestToYaml(t *testing.T) {
	type testCase struct {
		v      any
		result string
	}

	testCases := []testCase{
		{v: "a", result: "v: a"},
		{v: 1, result: "v: 1"},
		{v: "1", result: "v: '1'"},
		{v: "01", result: "v: '01'"},
		{v: "09", result: "v: '09'"},
	}
	j2 := newJinja2(t)

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v-%s", tc.v, tc.result), func(t *testing.T) {
			s, err := j2.RenderString("{{ m | to_yaml }}", WithGlobal("m", map[string]any{"v": tc.v}))
			assert.NoError(t, err)
			assert.Equal(t, tc.result+"\n", s)
		})
	}
}

func TestSha256(t *testing.T) {
	j2 := newJinja2(t)
	s, err := j2.RenderString("{{ 'test' | sha256 }}")
	assert.NoError(t, err)
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", s)
}

func TestSha256PrefixLength(t *testing.T) {
	j2 := newJinja2(t)
	s, err := j2.RenderString("{{ 'test' | sha256(6) }}")
	assert.NoError(t, err)
	assert.Equal(t, "9f86d0", s)
}

func TestGetVarAndRender(t *testing.T) {
	j2 := newJinja2(t, WithGlobals(map[string]any{
		"g1": "v1",
		"g2": "v2",
		"g3": map[string]any{
			"nested": "v3",
		},
		"list": []map[string]any{
			{"x": "i1"},
			{"x": "i2"},
		},
	}))

	type testCase struct {
		tmpl   string
		result string
	}

	tests := []testCase{
		{tmpl: "{{ get_var('missing') }}", result: "None"},
		{tmpl: "{{ get_var('missing', 'default') }}", result: "default"},
		{tmpl: "{{ get_var(['missing']) }}", result: "None"},
		{tmpl: "{{ get_var(['missing'], 'default') }}", result: "default"},
		{tmpl: "{{ get_var(['missing1', 'missing2'], 'default') }}", result: "default"},
		{tmpl: "{{ get_var(['g1', 'missing']) }}", result: "v1"},
		{tmpl: "{{ get_var(['missing', 'g1']) }}", result: "v1"},
		{tmpl: "{{ get_var(['g1', 'g2']) }}", result: "v1"},
		{tmpl: "{{ get_var(['g2', 'g1']) }}", result: "v2"},
		{tmpl: "{{ get_var('list') }}", result: `[{'x': 'i1'}, {'x': 'i2'}]`},
		{tmpl: "{{ get_var('list[0].x') }}", result: "i1"},
		{tmpl: "{% set loc = 'l1' %}{{ get_var('loc') }}", result: "l1"},
		// TODO test for this when https://github.com/pallets/jinja/issues/1478 gets fixed
		// {tmpl: "{% for e in list %}{{ get_var('e.x') }}{% endfor %}", result: ""},
		// same is before, but with a workaround to make 'e' a local variable
		{tmpl: "{% for e in list %}{% set e=e %}{{ get_var('e.x') }}{% endfor %}", result: "i1i2"},
		{tmpl: "{% set loc = 'l1' %}{{ '{{ loc }}' | render }}", result: "l1"},
		// TODO test for this when https://github.com/pallets/jinja/issues/1478 gets fixed
		// {tmpl: "{% for e in list %}{{ render('{{ e }}') }}{% endfor %}", result: ""},
		// same is before, but with a workaround to make 'e' a local variable
		{tmpl: "{% for e in list %}{% set e=e %}{{ render('{{ e.x }}') }}{% endfor %}", result: "i1i2"},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			s, err := j2.RenderString(tc.tmpl)
			assert.NoError(t, err)
			assert.Equal(t, tc.result, s)
		})
	}
}
