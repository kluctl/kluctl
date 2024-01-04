package uo

import (
	"github.com/ohler55/ojg/jp"
)

// implementation of Keyed from ojg/jp (json path)

// ensure we implement the interface
var _ jp.Keyed = &UnstructuredObject{}

func (uo *UnstructuredObject) ValueForKey(key string) (value any, has bool) {
	v, ok := uo.Object[key]
	return v, ok
}

func (uo *UnstructuredObject) SetValueForKey(key string, value any) {
	uo.Object[key] = value
}

func (uo *UnstructuredObject) RemoveValueForKey(key string) {
	delete(uo.Object, key)
}

func (uo *UnstructuredObject) Keys() []string {
	ret := make([]string, 0, len(uo.Object))
	for key, _ := range uo.Object {
		ret = append(ret, key)
	}
	return ret
}
