package mini_groupcache

import "mini-groupcache/testpb"

type PeerGetter interface {
	// Get 从 group 中查找缓存值
	// Get(group string, key string) ([]byte, error)
	
	// 使用 protobuf 节点之间通信
	Get(in *testpb.Request, out *testpb.Response) error
}

type PeerPicker interface { 
	// PickPeer 根据给定的 key 选择对应的节点
	PickPeer(key string) (peer PeerGetter, ok bool)
}


