package uo

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/yaml"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type UnstructuredObject struct {
	Object map[string]interface{} `yaml:"object,omitempty,inline"`
}

func (uo *UnstructuredObject) MarshalYAML() (interface{}, error) {
	return &uo.Object, nil
}

func (uo *UnstructuredObject) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&uo.Object)
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

func FromString(s string) (*UnstructuredObject, error) {
	o := New()
	err := yaml.ReadYamlString(s, &o.Object)
	if err != nil {
		return nil, err
	}
	return o, nil
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
		log.Fatal(err)
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
