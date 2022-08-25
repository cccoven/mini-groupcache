package singleflight

import "sync"

/**
缓存雪崩：缓存在同一时刻全部失效，造成瞬时DB请求量大、压力骤增，引起雪崩。缓存雪崩通常因为缓存服务器宕机、缓存的 key 设置了相同的过期时间等引起。
缓存击穿：一个存在的key，在缓存过期的一刻，同时有大量的请求，这些请求都会击穿到 DB ，造成瞬时 DB 请求量大、压力骤增。
缓存穿透：查询一个不存在的数据，因为不存在则不会写到缓存中，所以每次都会去请求 DB，如果瞬间流量过大，穿透到 DB，导致宕机。

如果同时有大量的请求并发地请求同一个 key，如果该 key 不存在任意一个节点缓存中，那么这些请求全部都会打到 DB 中，这时候会造成缓存穿透。
此时可以进行一些操作来控制这些相同的请求只会实际去请求一次，并返回相同的结果给所有的请求。
*/

// call 实际执行请求的执行单位
type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

// Group 防穿透的主要结构，每个分组对应一个，这样就只限制了这个分组的请求
type Group struct {
	mu sync.Mutex
	m  map[string]*call // 存储对每个 key 的请求
}

// Do 请求进入
func (g *Group) Do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	/* ----- 临界区 ----- */

	// 第一个到达的请求初始化 map
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	// 第一个到达的请求直接跳过
	if c, ok := g.m[key]; ok {
		// 非第一个到达的请求，可以从该 group 的 map 中直接取出第一个到达过的执行单元，并释放互斥锁
		g.mu.Unlock()
		// 等待这个执行单元的 waitGroup 计数器变成 0 即请求完成信号
		// 执行单元请求完成后会将计数器减 1，并已经准备好了请求的返回结果
		c.wg.Wait()
		// 直接将结果返回即可
		return c.val, c.err
	}

	// 实例化一个真正的执行单位，为其分配内存以保存请求的返回值
	c := new(call)
	// 执行单元的 waitGroup 计数器加 1，并存储该 key 和执行单元，表示有一个 goroutine 拿到了实际的请求权
	c.wg.Add(1)
	g.m[key] = c

	/* ----- 临界区 ----- */
	g.mu.Unlock()

	// 开始执行实际请求
	// 请求的结果保存在这个 key 的实际执行单元中
	c.val, c.err = fn()
	// 请求完成后，waitGroup 计数器减 1，表示已经完成了对一个 key 的请求
	c.wg.Done()

	// 请求完成后，删除 map 中的 key 表示对这个 key 的一次请求完成了
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	// 如果在出了 delete 的临界区之后返回值之前，再有请求进来，那么又会进入上面的流程中
	// 但不会影响这个请求最终的返回结果，因为执行单元 c 属于这个 goroutine 的局部变量
	
	// 最后将实际请求的值返回
	return c.val, c.err
}
