package uo

import (
	"fmt"
)

func MergeStrMap(a map[string]string, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}

func CopyMergeStrMap(a map[string]string, b map[string]string) map[string]string {
	c := make(map[string]string)
	MergeStrMap(c, a)
	MergeStrMap(c, b)
	return c
}

func GetChild(parent interface{}, key interface{}) (interface{}, bool, error) {
	if m, ok := parent.(map[string]interface{}); ok {
		keyStr, ok := key.(string)
		if !ok {
			return nil, false, fmt.Errorf("key is not a string")
		}
		v, found := m[keyStr]
		return v, found, nil
	} else if l, ok := parent.([]interface{}); ok {
		keyInt, ok := key.(int)
		if !ok {
			return nil, false, fmt.Errorf("key is not an int")
		}
		if keyInt < 0 || keyInt >= len(l) {
			return nil, false, fmt.Errorf("index out of bounds")
		}
		v := l[keyInt]
		return v, true, nil
	}
	return nil, false, fmt.Errorf("unknown parent type")
}

func SetChild(parent interface{}, key interface{}, value interface{}) error {
	if m, ok := parent.(map[string]interface{}); ok {
		keyStr, ok := key.(string)
		if !ok {
			return fmt.Errorf("key is not a string")
		}
		m[keyStr] = value
		return nil
	} else if l, ok := parent.([]interface{}); ok {
		keyInt, ok := key.(int)
		if !ok {
			return fmt.Errorf("key is not an int")
		}
		l[keyInt] = value
		return nil
	}
	return fmt.Errorf("unknown parent type")
}
