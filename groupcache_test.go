package mini_groupcache

import (
	"fmt"
	"log"
	"testing"
)

var db = map[string]string{
	"张三": "1",
	"李四": "2",
	"王五": "3",
}

func TestGroup_Get(t *testing.T) {
	loadCounts := make(map[string]int, len(db))

	group := NewGroup("scores", 2<<10, GetterFunc(func(key string) ([]byte, error) {
		// 模拟从远程数据库请求数据并返回
		log.Println("[SlowDB] search key ", key)
		if v, ok := db[key]; ok {
			if _, ok := loadCounts[key]; !ok {
				loadCounts[key] = 0
			}
			loadCounts[key] += 1

			return []byte(v), nil
		}

		return nil, fmt.Errorf("%s is not exist", key)
	}))

	view, err := group.Get("张三")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(view.String()) // "1"

	for k, v := range db {
		if view, err := group.Get(k); err != nil || view.String() != v {
			t.Fatal("远程获取数据源失败")
		}

		if _, err := group.Get(k); err != nil || loadCounts[k] > 1 {
			// 如果加载次数大于 1 就表示没有命中缓存
			t.Fatalf("cache %s miss", k)
		} // cache hit
	}

	if view, err := group.Get("unknown"); err == nil {
		t.Fatalf("the value of unknow should be empty, but %s got", view)
	}
}
