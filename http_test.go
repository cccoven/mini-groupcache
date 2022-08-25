package mini_groupcache

import (
	"fmt"
	"log"
	"net/http"
	"testing"
)

func TestHTTPPool_ServeHTTP(t *testing.T) {
	NewGroup("scores", 2<<10, GetterFunc(func(key string) ([]byte, error) {
		// 模拟从远程数据库请求数据并返回
		log.Println("[SlowDB] search key ", key)
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s is not exist", key)
	}))
	
	addr := "localhost:9999"
	peers := NewHTTPPool(addr)
	log.Println("groupcache is running at ", addr)
	// 将实现了 ServeHTTP 方法的接口体传给 http.ListenAndServe 以接管请求
	log.Fatal(http.ListenAndServe(addr, peers))
}
