package localcache

import "time"

type Iterator struct {
	Val    interface{} // 实际存储的对象
	Expire int64       // 过期时间，如果设置0，则表示不过期
}

// Expired 判断缓存是否过期
func (i *Iterator) Expired(e ...int64) bool {
	if i.Expire == 0 {
		return false
	}
	if len(e) != 0 {
		return e[0] > i.Expire
	}
	return time.Now().UnixNano() > i.Expire
}

type kv struct {
	key   string
	value interface{}
}
