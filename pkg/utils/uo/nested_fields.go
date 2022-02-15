package uo

import "fmt"

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
		if ok {
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
			return SetChild(o, i, l)
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
		return "", false, fmt.Errorf("value at %s is not a string", KeyListToJsonPath(keys))
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
	i, ok := v.(int64)
	if !ok {
		if i2, ok := v.(int); ok {
			i = int64(i2)
		} else {
			return 0, false, fmt.Errorf("value at %s is not an int", KeyListToJsonPath(keys))
		}
	}
	return i, true, nil
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
		return nil, false, fmt.Errorf("value at %s is not a slice", KeyListToJsonPath(keys))
	}
	return l, true, nil
}

func (uo *UnstructuredObject) GetNestedObject(keys ...interface{}) (*UnstructuredObject, bool, error) {
	a, found, err := uo.GetNestedField(keys...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
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
		return nil, false, fmt.Errorf("value at %s is not a map", KeyListToJsonPath(keys))
	}
	ret := make(map[string]string)
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, false, fmt.Errorf("value at %s.%s is not a string", KeyListToJsonPath(keys), k)
		}
		ret[k] = s
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
