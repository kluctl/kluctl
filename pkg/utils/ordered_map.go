package utils

type orderedMapEntry struct {
	k string
	v interface{}
}

type OrderedMap struct {
	m map[string]int
	l []orderedMapEntry
}

func (s *OrderedMap) Set(k string, v interface{}) bool {
	if s.m == nil {
		s.m = map[string]int{k: 0}
	} else {
		if _, ok := s.m[k]; ok {
			return false
		}
		s.m[k] = len(s.l)
	}
	s.l = append(s.l, orderedMapEntry{k: k, v: v})
	return true
}

func (s *OrderedMap) Has(v string) bool {
	_, ok := s.m[v]
	return ok
}

func (s *OrderedMap) Get(v string) (interface{}, bool) {
	i, ok := s.m[v]
	if !ok {
		return nil, ok
	}
	return s.l[i].v, true
}

func (s *OrderedMap) ListKeys() []string {
	l := make([]string, len(s.l))
	for i, e := range s.l {
		l[i] = e.k
	}
	return l
}

func (s *OrderedMap) ListValues() []interface{} {
	l := make([]interface{}, len(s.l))
	for i, e := range s.l {
		l[i] = e.v
	}
	return l
}
