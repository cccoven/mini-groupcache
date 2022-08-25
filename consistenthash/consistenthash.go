package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

// Map 是一致性哈希算法的主结构
// 什么是一致性哈希算法参考：https://www.zsythink.net/archives/1182
type Map struct {
	hash     Hash           // 哈希函数，用于计算 key
	replicas int            // 虚拟节点倍数，虚拟节点越多，哈希环的节点分布更均匀，数据也分配得更均匀，查找节点的时间也能优化
	keys     []int          // 哈希环 keys
	hashMap  map[int]string // 虚拟节点与真实节点的映射表
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		// hash 函数采用依赖注入的方式，允许替换成自己的哈希函数
		hash: fn,
		// 允许自定义虚拟节点倍数
		replicas: replicas,
		hashMap:  make(map[int]string),
	}

	if m.hash == nil {
		// 没有依赖注入时采用的默认哈希算法
		m.hash = crc32.ChecksumIEEE
	}

	return m
}

// Add 向哈希环中插入节点
// keys 允许传入多个真实节点的名称（通常使用分布式节点的名称/编号/IP地址）
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 对每一个真实节点 key 生成 m.replicas 个虚拟节点
		// 如：真实节点 6/4/2 生成虚拟节点 6/16/26、4/14/24、2/12/22
		for i := 0; i < m.replicas; i++ {
			// 基于真实节点的名称创建 m.replicas 个虚拟节点
			k := strconv.Itoa(i) + key
			hash := int(m.hash([]byte(k)))
			// 将所有虚拟节点保存到 m.keys
			m.keys = append(m.keys, hash)
			// 将每个虚拟节点存到映射表中，每个虚拟节点都对应真实节点
			// 如：6 -> 6、16 -> 6、26 -> 6
			m.hashMap[hash] = key
		}
	}
	// 将虚拟节点升序排序
	// 如：2, 4, 6, 12, 14, 16...
	sort.Ints(m.keys)
}

func (m *Map) Get(key string) string {
	if m.IsEmpty() {
		return ""
	}

	hash := int(m.hash([]byte(key)))

	// 该方法一般用于从一个已经排序的数组中找到某个值所对应的索引，或者从数组中找到满足某个条件的最小索引
	// 使用这个方法，就实现了哈希环上按顺时针找到最接近的节点的功能
	// 如：查找节点 8 会被定位到虚拟节点列表中第一个大于等于 8 的元素，即虚拟节点 12，对应的下标是 3
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	if idx == len(m.keys) {
		idx = 0
	}

	// 用匹配到的虚拟节点从映射表中找到对应的真实节点
	// 如：虚拟节点 12 映射到真实节点 2（Add 方法中添加的映射）
	return m.hashMap[m.keys[idx]]
}

// Remove 删除节点及其对应的虚拟节点
func (m *Map) Remove(key string) {
	for i := 0; i < m.replicas; i++ {
		k := strconv.Itoa(i) + key
		hash := m.hash([]byte(k))
		// sort.SearchInts 在一串有序的 int 数组中找到给定的值下标
		idx := sort.SearchInts(m.keys, int(hash))
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		delete(m.hashMap, int(hash))
	}
	sort.Ints(m.keys)
}

func (m *Map) IsEmpty() bool {
	return len(m.keys) == 0
}
