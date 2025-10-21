package localcache

import (
	"fmt"

	"hytera.com/ncp/pkg/options"
)

// Config 本地缓存配置结构体
type Config struct {
	capture func(key string, value interface{}) // 缓存数据删除捕获函数，当缓存项被删除时会调用此函数

	member map[string]Iterator // 成员映射，存储不同类型的缓存迭代器
}

// SetCapture 设置缓存删除捕获函数的配置选项
func SetCapture(capture func(key string, value interface{})) options.Option {
	return func(c interface{}) {
		c.(*Config).capture = capture
	}
}

// SetMember 设置初始化存储的成员对象
func SetMember(m map[string]Iterator) options.Option {
	return func(c interface{}) {
		c.(*Config).member = m
	}
}

// NewConfig 创建一个新的本地缓存配置实例
func NewConfig(opts ...options.Option) *Config {
	c := &Config{
		capture: func(k string, v interface{}) {
			fmt.Printf("delete k:%s v:%v\n", k, v)
		},
	}

	// 应用所有配置选项
	for _, opt := range opts {
		opt(c)
	}

	return c
}
