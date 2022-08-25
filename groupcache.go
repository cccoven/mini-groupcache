package mini_groupcache

import (
	"fmt"
	"log"
	"mini-groupcache/singleflight"
	"mini-groupcache/testpb"
	"sync"
)

// Getter 当缓存值不存在时，调用 Get 方法从其它数据源获取数据（文件、数据库、网络等）
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现 Getter 接口
// 函数类型实现某一个接口，称之为接口型函数，方便使用者在调用时既能够传入函数作为参数，也能够传入实现了该接口的结构体作为参数
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group 是一个缓存的命名空间
type Group struct {
	name      string
	getter    Getter // 缓存未命中时执行的回调用来获取数据源
	mainCache cache  // 并发安全的缓存

	peers  PeerPicker          // 分组内维护当前的节点信息（节点为 HTTPPool 结构）
	loader *singleflight.Group // 分组内控制并发的相同请求只会实际去请求一次
}

var (
	mu sync.Mutex
	// groups 全局变量用来存储所有的 group，以 name 作为分组名
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil getter")
	}

	// 初始化分组时要加上互斥锁，因为它们都操作了同一个全局变量 groups
	mu.Lock()
	defer mu.Unlock()

	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}

	groups[name] = g

	return g
}

func GetGroup(name string) *Group {
	return groups[name]
}

// RegisterPeers 将节点信息挂载到分组上
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// Get 获取缓存值
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 收到客户端或其它节点的请求，现在本地（自身节点）查找该 key 是否存在
	// 如果有多个相同的并发请求，同时读本地的缓存是被允许的
	if v, ok := g.mainCache.get(key); ok {
		log.Println("cache hit")
		return v, nil
	}

	// 本地不存在该值，尝试向其它节点查找
	return g.load(key)
}

// load 缓存没命中时，根据 getter 加载数据源到缓存里
// func (g *Group) load(key string) (value ByteView, err error) {
// 	// 在节点启动时，已经将哈希环上的节点信息都挂载到了这个分组上了
// 	if g.peers != nil {
// 		// 开始根据 key 从哈希环上寻找到对应的节点
// 		if peer, ok := g.peers.PickPeer(key); ok {
// 			// 找到了目标远程节点，开始向这个远程节点请求数据
// 			if value, err = g.getFromPeer(peer, key); err == nil {
// 				return value, nil
// 			}
// 			log.Println("[Groupcache] Failed to get from peer", err)
// 		}
// 	}
//
// 	// 找到的节点是自身或是没有找到其它节点或是没有存储其它节点，则直接调用定义分组时传入的 Getter 从其它数据源获取数据
// 	return g.getLocally(key)
// }

// load 缓存没命中时，根据 getter 加载数据源到缓存里
func (g *Group) load(key string) (value ByteView, err error) {
	// 缓存不存在时开始向其它节点或本地 Getter 查找，保证只会有一个实际的查找
	view, err := g.loader.Do(key, func() (any, error) {
		// 在节点启动时，已经将哈希环上的节点信息都挂载到了这个分组上了
		if g.peers != nil {
			// 开始根据 key 从哈希环上寻找到对应的节点
			if peer, ok := g.peers.PickPeer(key); ok {
				// 找到了目标远程节点，开始向这个远程节点请求数据
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[Groupcache] Failed to get from peer", err)
			}
		}

		// 找到的节点是自身或是没有找到其它节点或是没有存储其它节点，则直接调用定义分组时传入的 Getter 从其它数据源获取数据
		return g.getLocally(key)
	})
	if err != nil {
		return
	}

	return view.(ByteView), nil
}

// getFromPeer 从远程节点获取数据
// func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
// 	// 开始向远程节点发起 http 请求
// 	bytes, err := peer.Get(g.name, key)
// 	if err != nil {
// 		return ByteView{}, err
// 	}
//
// 	return ByteView{b: bytes}, nil
// }

// getFromPeer 从远程节点获取数据（使用 protobuf 通信）
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	in := &testpb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &testpb.Response{}
	// 开始向远程节点发起 http 请求
	err := peer.Get(in, res)
	if err != nil {
		return ByteView{}, err
	}

	return ByteView{b: res.Value}, nil
}

// load 缓存没命中时，根据用户给定的 getter 加载数据源到缓存里
// func (g *Group) load(key string) (ByteView, error) {
// 	return g.getLocally(key)
// }

// getLocally 实际调用 getter，并将值加入 cache
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	// 将数据源复制一份，不影响原来的数据源
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCate(key, value)

	return value, nil
}

// populateCate 将值加入缓存
func (g *Group) populateCate(key string, value ByteView) {
	g.mainCache.add(key, value)
}
