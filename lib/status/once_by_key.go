package status

import "sync"

type OnceByKey struct {
	m sync.Map
}

func (o *OnceByKey) Do(key string, cb func()) {
	o2, _ := o.m.LoadOrStore(key, &sync.Once{})
	o3 := o2.(*sync.Once)
	o3.Do(cb)
}
