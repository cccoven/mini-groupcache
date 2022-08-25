package main

import (
	"flag"
	"fmt"
	"log"
	mini_groupcache "mini-groupcache"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *mini_groupcache.Group {
	return mini_groupcache.NewGroup("scores", 2<<10, mini_groupcache.GetterFunc(func(key string) ([]byte, error) {
		log.Println("[SlowDB] search key", key)
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	}))
}

// startAPIServer 接收客户端请求
func startAPIServer(apiAddr string, group *mini_groupcache.Group) {
	http.Handle("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析客户端传来的 key，并开始查找
		key := r.URL.Query().Get("key")
		view, err := group.Get(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(view.ByteSlice())
	}))
	log.Println("frontend server is running at ", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func startCacheServer(addr string, addrs []string, group *mini_groupcache.Group) {
	// 创建当前节点服务端
	peers := mini_groupcache.NewHTTPPool(addr)
	// 将所有的节点都加入到哈希环上去并在当前节点维护这个哈希环
	peers.Set(addrs...)
	// 将当前节点的信息（包括自身的地址、维护的哈希、其它节点的请求地址等）存储到当前节点创建的分组中
	group.RegisterPeers(peers)
	log.Println("groupcache is running at", addr)
	// 当前节点开启 http 服务，当前节点实现了 ServeHTTP 方法，发来的请求会被接管
	// 这个 http 服务用于接收节点与节点直接的请求
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

/**
分布式节点流程：
	1）当前节点接收到客户端/远程节点的请求；
	2）在本地（当前节点）查找该 key 是否存在；
		2.1）存在：  返回给客户端/远程节点，流程结束。
		2.2）不存在：从哈希环上找出这个 key 对应的真实节点地址，进入流程 3；
	3）向目标真实节点发起请求，重新进入流程 1。

使用一致性哈希选择节点                    是                        是
    |-----> 哈希环上查找出是否是远程节点 -----> HTTP 客户端访问远程节点 --> 成功？-----> 服务端返回返回值
                    |  否                                    ↓  否
                    |----------------------------> 回退到本地节点处理。
*/
func main() {
	// 一个程序入口，代表一个分布式的节点
	var port int
	var api bool

	flag.IntVar(&port, "port", 8001, "Groupcache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	apiAddr := "http://localhost:9999" // 客户端调用端口
	addrMap := map[int]string{         // 其它节点列表
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	// 为节点创建一个分组
	group := createGroup()
	if api {
		// 为客户端提供调用服务
		go startAPIServer(apiAddr, group)
	}
	// 运行节点
	startCacheServer(addrMap[port], []string(addrs), group)
}
