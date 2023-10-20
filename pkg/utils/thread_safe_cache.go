package utils

import "sync"

type ThreadSafeCache[K comparable, V any] struct {
	mutex sync.Mutex
	cache map[K]*cacheEntry[V]
}

type cacheEntry[V any] struct {
	value V
	err   error
}

func (c *ThreadSafeCache[K, V]) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	clear(c.cache)
}

func (c *ThreadSafeCache[K, V]) ForEach(cb func(k K, v V)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for k, v := range c.cache {
		if v.err != nil {
			continue
		}
		cb(k, v.value)
	}
}

func (c *ThreadSafeCache[K, V]) Get(key K, f func() (V, error)) (V, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.cache == nil {
		c.cache = make(map[K]*cacheEntry[V])
	}

	e, ok := c.cache[key]
	if ok {
		return e.value, e.err
	}
	e = &cacheEntry[V]{}
	e.value, e.err = f()
	c.cache[key] = e

	return e.value, e.err
}

type ThreadSafeMultiCache[K comparable, V any] struct {
	cache ThreadSafeCache[string, *ThreadSafeCache[K, V]]
}

func (c *ThreadSafeMultiCache[K, V]) Get(cacheKey string, key K, f func() (V, error)) (V, error) {
	cache, _ := c.cache.Get(cacheKey, func() (*ThreadSafeCache[K, V], error) {
		return &ThreadSafeCache[K, V]{}, nil
	})
	return cache.Get(key, f)
}
