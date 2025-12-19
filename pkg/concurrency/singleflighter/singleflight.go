package singleflighter

import (
	"sync"

	"golang.org/x/sync/singleflight"
)

// 全局变量用于实现单例模式
var (
	once sync.Once           // once 确保 g 只被初始化一次
	g    *singleflight.Group // g 是全局共享的 singleflight.Group 实例
)

// DefaultSingleFlight 返回全局默认的 singleflight.Group 实例
// 使用 sync.Once 确保在并发环境下只创建一次实例，提供单例访问
func DefaultSingleFlight() *singleflight.Group {
	once.Do(func() {
		g = new(singleflight.Group)
	})
	return g
}

// NewSingleFlight 创建并返回一个新的 singleflight.Group 实例
// 每次调用都会创建新的实例，适用于需要多个独立分组的场景
func NewSingleFlight() *singleflight.Group {
	return new(singleflight.Group)
}
