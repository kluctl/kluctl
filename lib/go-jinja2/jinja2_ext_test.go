package jinja2

import (
	"fmt"
	"os"
	"strings"

	"github.com/stretchr/testify/assert"
)

func (suite *Jinja2TestSuite) TestToYaml() {
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
	j2 := suite.newJinja2()

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("%v-%s", tc.v, tc.result), func() {
			s, err := j2.RenderString("{{ m | to_yaml }}", WithGlobal("m", map[string]any{"v": tc.v}))
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.result+"\n", s)
		})
	}
}

func (suite *Jinja2TestSuite) TestSha256() {
	j2 := suite.newJinja2()
	s, err := j2.RenderString("{{ 'test' | sha256 }}")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", s)
}

func (suite *Jinja2TestSuite) TestSha256PrefixLength() {
	j2 := suite.newJinja2()
	s, err := j2.RenderString("{{ 'test' | sha256(6) }}")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "9f86d0", s)
}

func (suite *Jinja2TestSuite) TestGetVarAndRender() {
	j2 := suite.newJinja2(WithGlobals(map[string]any{
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
		suite.Run(fmt.Sprintf("%d", i), func() {
			s, err := j2.RenderString(tc.tmpl)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.result, s)
		})
	}
}

func (suite *Jinja2TestSuite) TestLoadLatestGitSha() {
	cwd, _ := os.Getwd()
	j2 := suite.newJinja2(WithSearchDir(cwd))
	s, err := j2.RenderString("sha: {{ load_latest_git_sha('./README.md') }}")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(s, "sha: "))
	assert.Equal(suite.T(), 5+40, len(s)) // 5 for "sha: ", 40 for sha length (from `man git-rev-parse`: "The full SHA-1 object name (40-byte hexadecimal string)").

	s, err = j2.RenderString("sha: {{ load_latest_git_sha('./README.md', 6) }}")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(s, "sha: "))
	assert.Equal(suite.T(), 5+6, len(s)) // 5 for "sha: ", 6 for sha length

}
