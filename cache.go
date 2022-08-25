package mini_groupcache

import (
	"mini-groupcache/lru"
	"sync"
)

// cache 封装 lru 的缓存，在其基础上提供互斥锁保证并发安全
type cache struct {
	mu         sync.Mutex // 同步化，实现并发安全的缓存
	lru        *lru.Cache // 使用 lru 缓存作为引擎
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock() // goroutine 到来时，加上互斥锁进入临界区
	defer c.mu.Unlock()

	if c.lru == nil { // 惰性载入缓存引擎
		c.lru = lru.NewCache(c.cacheBytes, nil)
	}

	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		return
	}

	v, ok := c.lru.Get(key)
	if !ok {
		return
	}

	// ByteView 实现了 Value 接口，返回的 v 是一个接口类型，这里可以直接对 v 使用断言
	return v.(ByteView), true
}

func (c *cache) removeOldest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.lru != nil {
		c.lru.RemoveOldest()
	}
}
