package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseEnvConfigSets_Prefixes(t *testing.T) {
	t.Setenv("PREFIX_A", "a")
	t.Setenv("PREFIX_B", "b")
	t.Setenv("PREFIX2_C", "c")
	assert.Equal(t, map[int]map[string]string{}, ParseEnvConfigSets("DUMMY"))
	assert.Equal(t, map[int]map[string]string{-1: {"A": "a", "B": "b"}}, ParseEnvConfigSets("PREFIX"))
	assert.Equal(t, map[int]map[string]string{-1: {"C": "c"}}, ParseEnvConfigSets("PREFIX2"))
}

func TestParseEnvConfigSets_Indexes(t *testing.T) {
	t.Setenv("PREFIX_A", "a")
	t.Setenv("PREFIX_0_A", "a0")
	t.Setenv("PREFIX_0_B", "b0")
	t.Setenv("PREFIX_1_A", "a1")
	assert.Equal(t, map[int]map[string]string{-1: {"A": "a"}, 0: {"A": "a0", "B": "b0"}, 1: {"A": "a1"}}, ParseEnvConfigSets("PREFIX"))
}

func TestParseEnvConfigList(t *testing.T) {
	t.Setenv("PREFIX", "a")
	assert.Equal(t, map[int]string{-1: "a"}, ParseEnvConfigList("PREFIX"))

	t.Setenv("PREFIX_0", "b")
	t.Setenv("PREFIX_1", "c")
	assert.Equal(t, map[int]string{-1: "a", 0: "b", 1: "c"}, ParseEnvConfigList("PREFIX"))
}
