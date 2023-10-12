package utils

type orderedMapEntry[T any] struct {
	k string
	v T
}

type OrderedMap[T any] struct {
	m map[string]int
	l []orderedMapEntry[T]
}

func (s *OrderedMap[T]) Set(k string, v T) bool {
	if s.m == nil {
		s.m = map[string]int{k: 0}
	} else {
		if _, ok := s.m[k]; ok {
			return false
		}
		s.m[k] = len(s.l)
	}
	s.l = append(s.l, orderedMapEntry[T]{k: k, v: v})
	return true
}

func (s *OrderedMap[T]) SetMultiple(k []string, v T) {
	for _, x := range k {
		s.Set(x, v)
	}
}

func (s *OrderedMap[T]) Has(v string) bool {
	_, ok := s.m[v]
	return ok
}

func (s *OrderedMap[T]) Get(v string) (T, bool) {
	i, ok := s.m[v]
	if !ok {
		var n T
		return n, ok
	}
	return s.l[i].v, true
}

func (s *OrderedMap[T]) ListKeys() []string {
	l := make([]string, len(s.l))
	for i, e := range s.l {
		l[i] = e.k
	}
	return l
}

func (s *OrderedMap[T]) ListValues() []T {
	l := make([]T, len(s.l))
	for i, e := range s.l {
		l[i] = e.v
	}
	return l
}

func (s *OrderedMap[T]) Merge(other *OrderedMap[T]) {
	for _, e := range other.l {
		s.Set(e.k, e.v)
	}
}
