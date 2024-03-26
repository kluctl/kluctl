package uo

import (
	"fmt"
	"reflect"
	"regexp"
)

func (uo *UnstructuredObject) GetNestedField(keys ...interface{}) (interface{}, bool, error) {
	var o interface{} = uo.Object
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

func (uo *UnstructuredObject) SetNestedField(value interface{}, keys ...interface{}) error {
	var o interface{} = uo.Object

	for _, field := range keys[:len(keys)-1] {
		val, ok, err := GetChild(o, field)
		if err != nil {
			return err
		}
		if ok && val != nil {
			o = val
		} else {
			newVal := make(map[string]interface{})
			err = SetChild(o, field, newVal)
			if err != nil {
				return err
			}
			o = newVal
		}
	}
	return SetChild(o, keys[len(keys)-1], value)
}

func (uo *UnstructuredObject) RemoveNestedField(keys ...interface{}) error {
	var o interface{} = uo.Object

	for _, field := range keys[:len(keys)-1] {
		val, ok, err := GetChild(o, field)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		o = val
	}
	last := keys[len(keys)-1]
	if l, ok := o.([]interface{}); ok {
		if i, ok := last.(int); ok {
			if i < 0 || i >= len(l) {
				return nil
			}
			l = append(l[:i], l[i+1:]...)
			return uo.SetNestedField(l, keys[0:len(keys)-1]...)
		} else {
			return fmt.Errorf("key is not an index")
		}
	} else if m, ok := o.(map[string]interface{}); ok {
		if s, ok := last.(string); ok {
			delete(m, s)
		} else {
			return fmt.Errorf("key is not a string")
		}
	}

	return nil
}

func (uo *UnstructuredObject) RemoveFieldsByPathRegex(path string) error {
	r, err := regexp.Compile(path)
	if err != nil {
		return err
	}

	var toDelete []KeyPath
	err = uo.NewIterator().IterateLeafs(func(it *ObjectIterator) error {
		jp := it.KeyPath().ToJsonPath()
		if r.MatchString(jp) {
			toDelete = append(toDelete, it.KeyPathCopy())
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, p := range toDelete {
		err = uo.RemoveNestedField(p...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uo *UnstructuredObject) GetNestedString(keys ...interface{}) (string, bool, error) {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf("value at %s is not a string", KeyPath(keys).ToJsonPath())
	}
	return s, true, nil
}

func (uo *UnstructuredObject) GetNestedInt(keys ...interface{}) (int64, bool, error) {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return 0, false, err
	}
	if !found {
		return 0, false, nil
	}
	vv := reflect.ValueOf(v)
	if vv.CanInt() {
		return vv.Int(), true, nil
	} else if vv.CanUint() {
		return int64(vv.Uint()), true, nil
	} else if vv.CanFloat() {
		return int64(vv.Float()), true, nil
	} else {
		return 0, false, fmt.Errorf("value at %s is not an int", KeyPath(keys).ToJsonPath())
	}
}

func (uo *UnstructuredObject) GetNestedList(keys ...interface{}) ([]interface{}, bool, error) {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	l, ok := v.([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("value at %s is not a slice", KeyPath(keys).ToJsonPath())
	}
	return l, true, nil
}

func (uo *UnstructuredObject) GetNestedStringList(keys ...interface{}) ([]string, bool, error) {
	l, found, err := uo.GetNestedList(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	ret := make([]string, len(l))
	for i, x := range l {
		s, ok := x.(string)
		if !ok {
			return nil, false, fmt.Errorf("value at index %s is not a slice of strings", KeyPath(keys).ToJsonPath())
		}
		ret[i] = s
	}
	return ret, true, nil
}

func (uo *UnstructuredObject) GetNestedObject(keys ...interface{}) (*UnstructuredObject, bool, error) {
	a, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	if a == nil {
		return nil, true, nil
	}

	m, ok := a.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("nested value is not a map")
	}
	return FromMap(m), true, nil
}

func (uo *UnstructuredObject) GetNestedObjectList(keys ...interface{}) ([]*UnstructuredObject, bool, error) {
	a, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	l, ok := a.([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("nested value is not a slice")
	}

	var ret []*UnstructuredObject
	for _, x := range l {
		x2, ok := x.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("nested value is not a slice of maps")
		}
		ret = append(ret, FromMap(x2))
	}
	return ret, true, nil
}

func (uo *UnstructuredObject) SetNestedObjectList(items []*UnstructuredObject, keys ...interface{}) error {
	var l []map[string]interface{}
	for _, i := range items {
		l = append(l, i.Object)
	}
	return uo.SetNestedField(l, keys...)
}

func (uo *UnstructuredObject) GetNestedObjectListNoErr(keys ...interface{}) []*UnstructuredObject {
	l, found, err := uo.GetNestedObjectList(keys...)
	if !found || err != nil {
		return nil
	}
	return l
}

func (uo *UnstructuredObject) GetNestedStringMapCopy(keys ...interface{}) (map[string]string, bool, error) {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("value at %s is not a map", KeyPath(keys).ToJsonPath())
	}
	ret := make(map[string]string)
	for k, v := range m {
		if v == nil {
			ret[k] = ""
		} else if s, ok := v.(string); ok {
			ret[k] = s
		} else {
			return nil, false, fmt.Errorf("value at %s.%s is not a string", KeyPath(keys).ToJsonPath(), k)
		}
	}

	return ret, true, nil
}

func (uo *UnstructuredObject) SetNestedFieldDefault(defaultValue interface{}, keys ...interface{}) error {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return err
	}
	if found && v != nil {
		return nil
	}
	return uo.SetNestedField(defaultValue, keys...)
}

func (uo *UnstructuredObject) GetNestedBool(keys ...interface{}) (bool, bool, error) {
	v, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return false, false, err
	}
	if !found {
		return false, true, nil
	}
	if x, ok := v.(bool); ok {
		return x, true, nil
	} else if x, ok := v.(*bool); ok {
		if x == nil {
			return false, true, nil
		}
		return *x, true, nil
	} else {
		return false, false, fmt.Errorf("field is not a bool")
	}
}
