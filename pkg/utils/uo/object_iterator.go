package uo

type ObjectIteratorFunc func(it *ObjectIterator) error
type ObjectIterator struct {
	path []interface{}
	keys []interface{}
	cb   ObjectIteratorFunc
}

func NewObjectIterator(o map[string]interface{}) *ObjectIterator {
	return &ObjectIterator{
		path: []interface{}{o},
	}
}

func (it *ObjectIterator) KeyPath() []interface{} {
	return it.keys
}

func (it *ObjectIterator) KeyPathCopy() []interface{} {
	ret := make([]interface{}, len(it.keys))
	copy(ret, it.keys)
	return ret
}

func (it *ObjectIterator) Key() interface{} {
	if len(it.keys) == 0 {
		return nil
	}
	return it.keys[len(it.keys)-1]
}

func (it *ObjectIterator) Parent() interface{} {
	if len(it.path) < 2 {
		return nil
	}
	return it.path[len(it.path)-2]
}

func (it *ObjectIterator) Value() interface{} {
	return it.path[len(it.path)-1]
}

func (it *ObjectIterator) SetValue(v interface{}) error {
	return SetChild(it.Parent(), it.Key(), v)
}

func (it *ObjectIterator) JsonPath() string {
	return KeyListToJsonPath(it.keys)
}

func (it *ObjectIterator) IterateLeafs(cb ObjectIteratorFunc) error {
	it.cb = cb
	return it.iterateInterface()
}

func (it *ObjectIterator) iterateInterface() error {
	value := it.Value()
	if _, ok := value.(map[string]interface{}); ok {
		return it.iterateMap()
	} else if _, ok := value.([]interface{}); ok {
		return it.iterateList()
	} else {
		return it.cb(it)
	}
}

func (it *ObjectIterator) iterateMap() error {
	m, ok := it.Value().(map[string]interface{})
	if !ok {
		panic("!ok")
	}

	for k, v := range m {
		it.path = append(it.path, v)
		it.keys = append(it.keys, k)
		err := it.iterateInterface()
		it.path = it.path[:len(it.path)-1]
		it.keys = it.keys[:len(it.keys)-1]

		if err != nil {
			return err
		}
	}
	return nil
}

func (it *ObjectIterator) iterateList() error {
	l, ok := it.Value().([]interface{})
	if !ok {
		panic("!ok")
	}

	for i, e := range l {
		it.path = append(it.path, e)
		it.keys = append(it.keys, i)
		err := it.iterateInterface()
		it.path = it.path[:len(it.path)-1]
		it.keys = it.keys[:len(it.keys)-1]
		if err != nil {
			return err
		}
	}
	return nil
}
