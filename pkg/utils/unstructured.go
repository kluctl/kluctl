package utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CopyObject(a map[string]interface{}) map[string]interface{} {
	var c map[string]interface{}
	err := DeepCopy(&c, &a)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func CopyUnstructured(u *unstructured.Unstructured) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: CopyObject(u.Object)}
}

func StructToObject(s interface{}) (map[string]interface{}, error) {
	b, err := WriteYamlBytes(s)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	err = ReadYamlBytes(b, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func ObjectToStruct(m map[string]interface{}, dst interface{}) error {
	b, err := WriteYamlBytes(m)
	if err != nil {
		return err
	}
	return ReadYamlBytes(b, dst)
}

func MergeObject(a map[string]interface{}, b map[string]interface{}) {
	if b == nil {
		b = map[string]interface{}{}
	}

	for key := range b {
		if _, ok := a[key]; ok {
			adict, adictOk := a[key].(map[string]interface{})
			bdict, bdictOk := b[key].(map[string]interface{})
			if adictOk && bdictOk {
				MergeObject(adict, bdict)
			} else {
				a[key] = b[key]
			}
		} else {
			a[key] = b[key]
		}
	}
}

func CopyMergeObjects(a map[string]interface{}, b map[string]interface{}) (map[string]interface{}, error) {
	c := CopyObject(a)
	MergeObject(c, b)
	return c, nil
}

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

func NestedMapSlice(o map[string]interface{}, fields ...string) ([]map[string]interface{}, bool, error) {
	a, found, err := unstructured.NestedSlice(o, fields...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	var ret []map[string]interface{}
	for _, x := range a {
		x2, ok := x.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("nested value is not a slice of maps")
		}
		ret = append(ret, x2)
	}
	return ret, true, nil
}

func NestedMapSliceNoErr(o map[string]interface{}, fields ...string) []map[string]interface{} {
	l, found, err := NestedMapSlice(o, fields...)
	if !found || err != nil {
		return nil
	}
	return l
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

func GetNestedChild(o interface{}, keys ...interface{}) (interface{}, bool, error) {
	for _, k := range keys {
		v, found, err := GetChild(o, k)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		o = v
	}
	return o, true, nil
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
