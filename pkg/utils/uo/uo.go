package uo

import (
	"encoding/json"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
)

type UnstructuredObject struct {
	Object map[string]interface{} `json:"object,omitempty,inline"`
}

func (uo *UnstructuredObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(uo.Object)
}

func (uo *UnstructuredObject) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &uo.Object)
}

func (uo *UnstructuredObject) IsZero() bool {
	return len(uo.Object) == 0
}

func New() *UnstructuredObject {
	return &UnstructuredObject{Object: map[string]interface{}{}}
}

func FromMap(o map[string]interface{}) *UnstructuredObject {
	return &UnstructuredObject{Object: o}
}

func FromUnstructured(u *unstructured.Unstructured) *UnstructuredObject {
	return FromMap(u.Object)
}

func (uo *UnstructuredObject) ToUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: uo.Object}
}

func FromStruct(o interface{}) (*UnstructuredObject, error) {
	b, err := yaml.WriteYamlBytes(o)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	err = yaml.ReadYamlBytes(b, &m)
	if err != nil {
		return nil, err
	}
	return FromMap(m), nil
}

func (uo *UnstructuredObject) ToStruct(out interface{}) error {
	b, err := yaml.WriteYamlBytes(uo.Object)
	if err != nil {
		return err
	}

	return yaml.ReadYamlBytes(b, out)
}

// ToMap will ensure that only plain go values are returned, meaning that all internal structs are converted
// to maps
func (uo *UnstructuredObject) ToMap() (map[string]any, error) {
	b, err := yaml.WriteYamlBytes(uo.Object)
	if err != nil {
		return nil, err
	}

	var ret map[string]any
	err = yaml.ReadYamlBytes(b, &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func FromString(s string) (*UnstructuredObject, error) {
	o := New()
	err := yaml.ReadYamlString(s, &o.Object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func FromStringMust(s string) *UnstructuredObject {
	o, err := FromString(s)
	if err != nil {
		panic(err)
	}
	return o
}

func FromFile(p string) (*UnstructuredObject, error) {
	o := New()
	err := yaml.ReadYamlFile(p, &o.Object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func FromStringMulti(s string) ([]*UnstructuredObject, error) {
	ifs, err := yaml.ReadYamlAllString(s)
	if err != nil {
		return nil, err
	}
	return fromAnyList(ifs)
}

func FromFileMulti(p string) ([]*UnstructuredObject, error) {
	ifs, err := yaml.ReadYamlAllFile(p)
	if err != nil {
		return nil, err
	}
	return fromAnyList(ifs)
}

func fromAnyList(ifs []any) ([]*UnstructuredObject, error) {
	var ret []*UnstructuredObject
	for _, i := range ifs {
		m, ok := i.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("object is not a map")
		}
		ret = append(ret, FromMap(m))
	}
	return ret, nil
}

func (uo *UnstructuredObject) Clone() *UnstructuredObject {
	var c map[string]interface{}
	err := copier.CopyWithOption(&c, &uo.Object, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		panic(err)
	}
	return FromMap(c)
}

func (uo *UnstructuredObject) Merge(other *UnstructuredObject) {
	MergeMap(uo.Object, other.Object)
}

func (uo *UnstructuredObject) MergeChild(child string, other *UnstructuredObject) {
	MergeMap(uo.Object, map[string]interface{}{
		child: other.Object,
	})
}

func (uo *UnstructuredObject) MergeCopy(other *UnstructuredObject) *UnstructuredObject {
	c := uo.Clone()
	c.Merge(other)
	return c
}

func (uo *UnstructuredObject) NewIterator() *ObjectIterator {
	return NewObjectIterator(uo.Object)
}

func (uo *UnstructuredObject) Clear() {
	uo.Object = make(map[string]interface{})
}

type uoParentKeyValue struct {
	parent interface{}
	key    interface{}
	value  interface{}
}

func (uo *UnstructuredObject) ReplaceKeys(oldKey string, newKey string) error {
	var toDelete []uoParentKeyValue
	var toSet []uoParentKeyValue
	err := uo.NewIterator().IterateLeafs(func(it *ObjectIterator) error {
		keyStr, ok := it.Key().(string)
		if ok && keyStr == oldKey {
			toDelete = append(toDelete, uoParentKeyValue{
				parent: it.Parent(),
				key:    it.Key(),
			})
			toSet = append(toSet, uoParentKeyValue{
				parent: it.Parent(),
				key:    newKey,
				value:  it.Value(),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, p := range toDelete {
		delete(p.parent.(map[string]interface{}), p.key.(string))
	}
	for _, p := range toSet {
		err = SetChild(p.parent, p.key, p.value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (uo *UnstructuredObject) ReplaceValues(oldValue interface{}, newValue interface{}) error {
	return uo.NewIterator().IterateLeafs(func(it *ObjectIterator) error {
		if reflect.DeepEqual(it.Value(), oldValue) {
			err := it.SetValue(newValue)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
