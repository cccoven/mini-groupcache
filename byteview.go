package mini_groupcache

// ByteView
type ByteView struct {
	b []byte // 存储真实的缓存值，使用 byte 类型可以存储任意类型的值，这个值是只读的	
}

// Len 实现 Value 接口
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回一个对 b 的拷贝，防止缓存值被外部修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
