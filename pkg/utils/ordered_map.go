package utils

type orderedMapEntry[K comparable, V any] struct {
	k K
	v V
}

type OrderedMap[K comparable, V any] struct {
	m map[K]int
	l []orderedMapEntry[K, V]
}

func (s *OrderedMap[K, V]) ForEach(cb func(k K, v V)) {
	for _, kv := range s.l {
		cb(kv.k, kv.v)
	}
}

func (s *OrderedMap[K, V]) Set(k K, v V) bool {
	if s.m == nil {
		s.m = map[K]int{k: 0}
	} else {
		if _, ok := s.m[k]; ok {
			return false
		}
		s.m[k] = len(s.l)
	}
	s.l = append(s.l, orderedMapEntry[K, V]{k: k, v: v})
	return true
}

func (s *OrderedMap[K, V]) SetMultiple(k []K, v V) {
	for _, x := range k {
		s.Set(x, v)
	}
}

func (s *OrderedMap[K, V]) Has(k K) bool {
	_, ok := s.m[k]
	return ok
}

func (s *OrderedMap[K, V]) Get(k K) (V, bool) {
	i, ok := s.m[k]
	if !ok {
		var n V
		return n, ok
	}
	return s.l[i].v, true
}

func (s *OrderedMap[K, V]) ListKeys() []K {
	l := make([]K, len(s.l))
	for i, e := range s.l {
		l[i] = e.k
	}
	return l
}

func (s *OrderedMap[K, V]) ListValues() []V {
	l := make([]V, len(s.l))
	for i, e := range s.l {
		l[i] = e.v
	}
	return l
}

func (s *OrderedMap[K, V]) Merge(other *OrderedMap[K, V]) {
	for _, e := range other.l {
		s.Set(e.k, e.v)
	}
}

func (s *OrderedMap[K, V]) Len() int {
	return len(s.l)
}
