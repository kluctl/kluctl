package uo

import (
	"fmt"
	"github.com/ohler55/ojg/jp"
	"regexp"
	"strings"
)

var isSimpleIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]+$`)

func KeyListToJsonPath(keys []interface{}) string {
	p := ""
	for _, k := range keys {
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

func (j *MyJsonPath) ListMatchingFields(o *UnstructuredObject) ([][]interface{}, error) {
	var ret [][]interface{}

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

func (j *MyJsonPath) Get(o interface{}) []interface{} {
	return j.exp.Get(o)
}

func (j *MyJsonPath) GetFirst(o interface{}) (interface{}, bool) {
	l := j.Get(o)
	if len(l) == 0 {
		return nil, false
	}
	return l[0], true
}

func (j *MyJsonPath) GetFirstMap(o interface{}) (map[string]interface{}, bool, error) {
	o, found := j.GetFirst(o)
	if !found {
		return nil, false, nil
	}
	m, ok := o.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("child is not a map")
	}
	return m, true, nil
}

func (j *MyJsonPath) GetFirstListOfMaps(o interface{}) ([]map[string]interface{}, bool, error) {
	o, found := j.GetFirst(o)
	if !found {
		return nil, false, nil
	}
	m, ok := o.([]map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("child is not a list of maps")
	}
	return m, true, nil
}

func (j *MyJsonPath) Del(o interface{}) error {
	return j.exp.Del(o)
}
