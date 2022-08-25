package mini_groupcache

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"mini-groupcache/consistenthash"
	"mini-groupcache/testpb"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_groupcache/"
	defaultReplicas = 50
)

// httpGetter 实现 PeerGetter 接口，用于与客户端通信
type httpGetter struct {
	baseURL string
}

// HTTPPool 实现服务端与服务端之间的通信
type HTTPPool struct {
	self string // 记录自己的地址（主机名/IP和端口号）
	// 节点间通信地址的前缀
	// 如 http://example.com/_groupcache/ 开头的请求就用于节点之间的通信，相当于一个路由标识
	basePath string

	mu    sync.Mutex
	peers *consistenthash.Map // 每个节点持有整个哈希环上的真实和虚拟节点，用来根据 key 选择相应的节点
	// 映射远程节点与对应的 httpGetter，每一个远程节点对应一个 httpGetter，因为 httpGetter 与远程节点的地址 baseURL 有关
	// 这里就是持有每个节点与之对应的 http 请求地址
	// 如：http://localhost:8001 -> http://localhost:8001/_groupcache/  http://localhost:8002 -> http://localhost:8002/_groupcache/
	httpGetters map[string]*httpGetter
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Get 在 httpGetter 上实现 PeerGetter 接口，用于从其它节点获取缓存值
// func (h *httpGetter) Get(group string, key string) ([]byte, error) {
// 	// 向远程节点发起请求很简单，就是将节点上存储的远程节点请求地址拼上 /<groupname>/<key> 并发送 GET 请求即可
// 	u := fmt.Sprintf(
// 		"%v%v/%v",
// 		h.baseURL,
// 		url.QueryEscape(group),
// 		url.QueryEscape(key),
// 	)
//
// 	// 每个节点在启动了都开启了自己 http 服务，即在前面 main.go 中 startCacheServer 方法里
// 	// 发送 http 请求，就会进入到目标节点自己的 ServeHTTP 方法中
// 	resp, err := http.Get(u)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
//
// 	if resp.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("server returned: %v", resp.Status)
// 	}
//
// 	bytes, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("reading response body: %v", err)
// 	}
//
// 	return bytes, nil
// }

// Get 在 httpGetter 上实现 PeerGetter 接口，用于从其它节点获取缓存值（使用 protobuf 通信）
func (h *httpGetter) Get(in *testpb.Request, out *testpb.Response) error {
	// 向远程节点发起请求很简单，就是将节点上存储的远程节点请求地址拼上 /<groupname>/<key> 并发送 GET 请求即可
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)

	// 每个节点在启动了都开启了自己 http 服务，即在前面 main.go 中 startCacheServer 方法里
	// 发送 http 请求，就会进入到目标节点自己的 ServeHTTP 方法中
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", resp.Status)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	// proto.Marshal 将字节编码成 protobuf 消息
	// proto.Unmarshal 将 protobuf 消息解码成字节
	// 可以利用这一点实现数据库实体类与 protobuf 结构体直接的转换
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}

// 这种写法先为 PeerGetter 接口创建一个地址，但不分配内存，如果给字段赋值会报错
// 在代码中判断 httpGetter 这个 struct 是否实现了 PeerGetter 接口，没有实现则报错
var _ PeerGetter = (*httpGetter)(nil)

// Set 在当前节点上存储其它节点的信息（加入哈希环），包括真实与虚拟节点、其它节点的请求地址
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 创建哈希环，默认创建 50 倍的虚拟节点
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 将真实节点加入哈希环
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))

	// 存储所有其它节点的服务请求地址
	// 如 http://localhost:8001 -> http://localhost:8001/_groupcache/
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer 实现了 PeerPicker 接口，用于从哈希环中选择一个节点
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 使用一致性哈希算法的查找，找出该 key 对应的真实节点
	peer := p.peers.Get(key)
	if peer != "" && peer != p.self {
		// 找到了目标远程节点且不是自身节点，返回该远程节点的请求地址，如 http://localhost:8002/_groupcache/
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}

	return nil, false
}

func (p *HTTPPool) Log(format string, v ...any) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) { // 前缀匹配不上
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}

	p.Log("%s %s", r.Method, r.URL.Path)

	// 通讯形式：example.com/<basepath>/<groupname>/<key>
	// 将 <groupname> 和 <key> 从路由中分离出来
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// 拿到分组名和 key，从缓存查找值
	groupName, key := parts[0], parts[1]
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "No such group: "+groupName, http.StatusNotFound)
		return
	}

	// 接收到了来自其它节点的请求，与发来请求的节点一样，进入查找缓存值的流程
	// 这里就形成了一个闭环
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用 protobuf 通信
	body, err := proto.Marshal(&testpb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取到值之后，写入到 response body 里
	w.Header().Set("Content-Type", "application/octet-stream")
	// w.Write(view.ByteSlice())
	w.Write(body)
}
