package lru

import (
	"fmt"
	"testing"
)

type String string

func (s String) Len() int {
	return len(s)
}

func TestCache(t *testing.T) {
	lru := NewCache(int64(0), nil)
	lru.Add("foo", String("bar"))
	
	fmt.Println(lru.Get("foo"))
	fmt.Println(lru.Get("foo2"))
}

func TestCache_RemoveOldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	cap := len(k1 + k2 + v1 + v2)
	cb := func(key string, value Value) {
		fmt.Println("淘汰后调用回调")
	}
	
	lru := NewCache(int64(cap), cb)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3))
	
	// 容量满了时，lru 会先淘汰最早进入的 key1
	if _, ok := lru.Get("key1"); ok || lru.Len() != 2 {
		t.Fatal("淘汰 key1 失败")
	}
}
