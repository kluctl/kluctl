package uo

import (
	"fmt"
	"github.com/ohler55/ojg/jp"
	"reflect"
	"regexp"
	"strings"
)

var isSimpleIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]+$`)

type KeyPath []interface{}

func (kl KeyPath) ToJsonPath() string {
	p := ""
	for _, k := range kl {
		if i, ok := k.(int); ok {
			p = fmt.Sprintf("%s[%d]", p, i)
		} else if s, ok := k.(string); ok {
			if isSimpleIdentifier.MatchString(s) {
				if p != "" {
					p += "."
				}
				p += s
			} else {
				if p == "" {
					p = "$"
				}
				if strings.Index(s, "\"") != -1 {
					p = fmt.Sprintf("%s['%s']", p, s)
				} else {
					p = fmt.Sprintf("%s[\"%s\"]", p, s)
				}
			}
		} else {
			if p == "" {
				p = "$"
			}
			p = fmt.Sprintf("%s[%v]", p, k)
		}
	}
	return p
}

type MyJsonPath struct {
	exp jp.Expr
}

func NewMyJsonPath(p string) (*MyJsonPath, error) {
	exp, err := jp.ParseString(p)
	if err != nil {
		return nil, err
	}
	return &MyJsonPath{
		exp: exp,
	}, nil
}

func NewMyJsonPathMust(p string) *MyJsonPath {
	j, err := NewMyJsonPath(p)
	if err != nil {
		panic(err)
	}
	return j
}

func (j *MyJsonPath) ListMatchingFields(o *UnstructuredObject) ([]KeyPath, error) {
	var ret []KeyPath

	o = o.Clone()
	magic := struct{}{}

	err := j.exp.Set(o.Object, magic)
	if err != nil {
		return nil, err
	}

	_ = o.NewIterator().IterateLeafs(func(it *ObjectIterator) error {
		if it.Value() == magic {
			var c []interface{}
			c = append(c, it.KeyPath()...)
			ret = append(ret, c)
		}
		return nil
	})

	return ret, nil
}

func (j *MyJsonPath) Get(o *UnstructuredObject) []interface{} {
	return j.exp.Get(o.Object)
}

func (j *MyJsonPath) GetFirst(o *UnstructuredObject) (interface{}, bool) {
	l := j.Get(o)
	if len(l) == 0 {
		return nil, false
	}
	return l[0], true
}

func (j *MyJsonPath) GetFromAny(o any) []interface{} {
	return j.exp.Get(o)
}

func (j *MyJsonPath) GetFirstFromAny(o any) (interface{}, bool) {
	l := j.GetFromAny(o)
	if len(l) == 0 {
		return nil, false
	}
	return l[0], true
}

func (j *MyJsonPath) GetFirstObject(o *UnstructuredObject) (*UnstructuredObject, bool, error) {
	x, found := j.GetFirst(o)
	if !found {
		return nil, false, nil
	}
	m, ok := getDict(x)
	if !ok {
		return nil, false, fmt.Errorf("child is not a map")
	}
	return FromMap(m), true, nil
}

func (j *MyJsonPath) GetFirstListOfObjects(o *UnstructuredObject) ([]*UnstructuredObject, bool, error) {
	x, found := j.GetFirst(o)
	if !found {
		return nil, false, nil
	}
	if x == nil {
		// nil is a valid list of zero elements, so treat it as 'found'
		return nil, true, nil
	}
	v := reflect.ValueOf(x)
	if v.Type().Kind() != reflect.Slice {
		return nil, false, fmt.Errorf("child is not a list")
	}

	var ret []*UnstructuredObject
	for i := 0; i < v.Len(); i++ {
		e := v.Index(i).Interface()
		m, ok := getDict(e)
		if !ok {
			return nil, false, fmt.Errorf("child is not a list of maps")
		}
		ret = append(ret, FromMap(m))
	}
	return ret, true, nil
}

func (j *MyJsonPath) Del(o *UnstructuredObject) error {
	return j.exp.Del(o.Object)
}

func (j *MyJsonPath) Set(o *UnstructuredObject, v any) error {
	return j.exp.Set(o.Object, v)
}

func (j *MyJsonPath) SetOne(o *UnstructuredObject, v any) error {
	return j.exp.SetOne(o.Object, v)
}
