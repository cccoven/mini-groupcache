package lru

import "container/list"

// Value 实现 Len() 方法来返回值占用的内存大小
type Value interface {
	Len() int
}

// entry 是双向链表节点数据（Value）的数据类型
type entry struct {
	key   string
	value Value
}

// Cache 采用 LRU 算法实现缓存，它暂时并不是并发安全的
type Cache struct {
	maxBytes int64      // 缓存最大容量
	nbytes   int64      // 当前缓存总容量
	ll       *list.List // 使用 Go 内置的双向链表实现 LRU 算法
	// 使用 map（哈希表）存储缓存数据，值是双向链表中节点的指针，这样就可以通过 O(1) 复杂度访问到对应的缓存值
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value) // 当一个对值被清除时执行（钩子），可选
}

func NewCache(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Add 新增/修改缓存值
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		// 要缓存的值已存在，将其移动到队首表示最近访问过
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry) // 取出值
		// 重新计算新的值所占用的内存
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		// 更新缓存值
		kv.value = value
	} else {
		// 要缓存的值不存在，将其加入到队首
		ele = c.ll.PushFront(&entry{key, value})
		// 加入 cache map 中，使这个 key 与实际存储在链表中的值形成一个映射并能快速访问到
		c.cache[key] = ele
		// 累加内存
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	
	if c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}
}

// Get 获取缓存值
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		// 缓存中查找到值则将其移动到队首并返回 Value
		c.ll.MoveToFront(ele)
		value = ele.Value.(*entry).value
		return value, true
	}

	return
}

// RemoveOldest 删除即缓存淘汰，从 LRU 链表队首移除最近最少访问的节点
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() // 取出队尾节点
	if ele == nil {
		return
	}

	c.ll.Remove(ele) // 删除队尾节点
	kv := ele.Value.(*entry)
	delete(c.cache, kv.key)                                // 从映射表中删除
	c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) // 释放内存

	// 如果传入了钩子函数就调用
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

// Len 返回缓存的键值对数量
func (c *Cache) Len() int {
	return c.ll.Len()
}
