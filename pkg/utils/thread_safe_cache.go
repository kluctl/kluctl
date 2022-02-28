package utils

import "sync"

type ThreadSafeCache struct {
	mutex sync.Mutex
	cache map[string]*cacheEntry
}

type cacheEntry struct {
	value interface{}
	err   error
}

func (c *ThreadSafeCache) Get(key string, f func() (interface{}, error)) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.cache == nil {
		c.cache = make(map[string]*cacheEntry)
	}

	e, ok := c.cache[key]
	if ok {
		return e.value, e.err
	}
	e = &cacheEntry{}
	e.value, e.err = f()
	c.cache[key] = e

	return e.value, e.err
}

type ThreadSafeMultiCache struct {
	cache ThreadSafeCache
}

func (c *ThreadSafeMultiCache) Get(cacheKey string, key string, f func() (interface{}, error)) (interface{}, error) {
	cache, _ := c.cache.Get(cacheKey, func() (interface{}, error) {
		return &ThreadSafeCache{}, nil
	})
	return cache.(*ThreadSafeCache).Get(key, f)
}
