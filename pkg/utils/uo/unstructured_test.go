package uo

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetChildMap(t *testing.T) {
	x := map[string]any{
		"a": 1,
	}

	_ = SetChild(x, "b", 42)
	assert.Equal(t, 1, x["a"])
	assert.Equal(t, 42, x["b"])

	v, _, _ := GetChild(x, "a")
	assert.Equal(t, 1, v)
	v, _, _ = GetChild(x, "b")
	assert.Equal(t, 42, v)
}

func TestSetGetChildUnstructuredObject(t *testing.T) {
	x := FromMap(map[string]any{
		"a": 1,
	})

	_ = SetChild(x, "b", 42)
	assert.Equal(t, 1, x.Object["a"])
	assert.Equal(t, 42, x.Object["b"])

	v, _, _ := GetChild(x, "a")
	assert.Equal(t, 1, v)
	v, _, _ = GetChild(x, "b")
	assert.Equal(t, 42, v)
}

func TestSetGetChildAnySlice(t *testing.T) {
	x := []any{
		1,
		111,
	}

	_ = SetChild(x, 1, 42)
	assert.Equal(t, 1, x[0])
	assert.Equal(t, 42, x[1])

	v, _, _ := GetChild(x, 0)
	assert.Equal(t, 1, v)
	v, _, _ = GetChild(x, 1)
	assert.Equal(t, 42, v)
}

func TestSetGetChildIntSlice(t *testing.T) {
	x := []int{
		1,
		111,
	}

	_ = SetChild(x, 1, 42)
	assert.Equal(t, 1, x[0])
	assert.Equal(t, 42, x[1])

	v, _, _ := GetChild(x, 0)
	assert.Equal(t, 1, v)
	v, _, _ = GetChild(x, 1)
	assert.Equal(t, 42, v)
}
