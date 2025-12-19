package ratelimit

import (
	"sync"
	"time"
)

// mutexLimiter 是基于互斥锁实现的限流器
type mutexLimiter struct {
	sync.Mutex               // 互斥锁，保护共享状态
	last       time.Time     // 上次请求的时间戳
	sleepFor   time.Duration // 需要睡眠的时间长度
	perRequest time.Duration // 每个请求之间的时间间隔 (per/rate)
	maxSlack   time.Duration // 最大松弛时间，等于 -1 * slack * perRequest (负数)
	clock      Clock         // 时钟接口，用于获取当前时间和睡眠
}

// newMutexBased 创建一个新的基于互斥锁的限流器
func newMutexBased(rate int, opts ...Option) *mutexLimiter {
	// 构建配置参数
	c := buildConfig(opts)
	// 计算每个请求应该间隔的时间
	perRequest := c.per / time.Duration(rate)
	l := &mutexLimiter{
		clock:      c.clock,
		perRequest: perRequest,
		maxSlack:   -1 * time.Duration(c.slack) * perRequest, // 最大松弛时间为负数，用于限制累积的负值
	}
	return l
}

// Take 阻塞以确保多次调用 Take 之间的平均时间符合 per/rate 的限制
func (t *mutexLimiter) Take() time.Time {
	// 加锁保护共享状态
	t.Lock()
	defer t.Unlock()

	// 获取当前时间
	now := t.clock.Now()

	// 如果是第一次请求，则直接允许
	if t.last.IsZero() {
		t.last = now
		return t.last
	}

	// 计算需要睡眠的时间：基于每个请求的预算时间和上次请求到现在的时间差
	// 因为请求可能比预算时间长，这个数值可能是负数，并且会在请求间累加
	t.sleepFor += t.perRequest - now.Sub(t.last)

	// 不应让 sleepFor 变得太负，因为这意味着服务在短时间内大幅减速后会获得更高的 RPS
	// maxSlack 是负数，用于限制 sleepFor 的最小值
	if t.sleepFor < t.maxSlack {
		t.sleepFor = t.maxSlack
	}

	// 如果需要睡眠，则执行睡眠
	if t.sleepFor > 0 {
		t.clock.Sleep(t.sleepFor)
		t.last = now.Add(t.sleepFor)
		t.sleepFor = 0 // 重置睡眠时间
	} else {
		t.last = now
	}

	return t.last
}
