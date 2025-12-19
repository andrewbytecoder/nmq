package ratelimit

import (
	"sync/atomic"
	"time"
)

// atomicInt64Limiter 是基于原子操作实现的限流器，使用 int64 存储时间戳
type atomicInt64Limiter struct {
	// lint:ignore U1000 Padding is unused but it is crucial to maintain performance
	// of this rate limiter in case of collocation with other frequently accessed memory.
	prepadding [64]byte // 缓存行前置填充，避免伪共享，缓存行大小为64字节
	state      int64    // 下一次允许请求的Unix纳秒时间戳

	// lint:ignore U1000 like prepadding.
	postpadding [56]byte // 缓存行后置填充，缓存行大小64 - state大小8 = 56字节，避免伪共享

	perRequest time.Duration // 每个请求之间的时间间隔 (per/rate)
	maxSlack   time.Duration // 最大松弛时间，等于 slack * perRequest
	clock      Clock         // 时钟接口，用于获取当前时间和睡眠
}

// newAtomicInt64Based 创建一个新的基于原子操作的int64限流器
func newAtomicInt64Based(rate int, opts ...Option) *atomicInt64Limiter {
	// 构建配置参数
	c := buildConfig(opts)
	// 计算每个请求应该间隔的时间
	perRequest := c.per / time.Duration(rate)
	l := &atomicInt64Limiter{
		perRequest: perRequest,
		maxSlack:   time.Duration(c.slack) * perRequest, // 最大松弛时间为 slack 倍的单个请求时间
		clock:      c.clock,
	}
	// 初始化状态为0
	atomic.StoreInt64(&l.state, 0)
	return l
}

// Take 阻塞以确保多次调用 Take 之间的平均时间符合 time.Second/rate 的限制
func (t *atomicInt64Limiter) Take() time.Time {
	var (
		newTimeOfNextPermissionIssue int64 // 计算出的下一次允许请求的时间戳
		now                          int64 // 当前时间的Unix纳秒时间戳
	)

	// 使用 CAS 循环直到成功更新状态
	for {
		// 获取当前时间的Unix纳秒时间戳
		now = t.clock.Now().UnixNano()
		// 原子加载下一次允许请求的时间戳
		timeOfNextPermissionIssue := atomic.LoadInt64(&t.state)

		// 根据不同情况计算下一次允许请求的时间
		switch {
		case timeOfNextPermissionIssue == 0 || (t.maxSlack == 0 && now-timeOfNextPermissionIssue > int64(t.perRequest)):
			// 如果是第一次调用或者最大松弛时间为0且距离上次调用时间超过单个请求时间
			// 则将下一次允许请求时间设置为现在
			newTimeOfNextPermissionIssue = now
		case t.maxSlack > 0 && now-timeOfNextPermissionIssue > int64(t.maxSlack)+int64(t.perRequest):
			// 如果距离上次调用时间很长，超过了最大松弛时间+单个请求时间
			// 限制累积时间为最大松弛时间，防止突发大量请求
			newTimeOfNextPermissionIssue = now - int64(t.maxSlack)
		default:
			// 正常情况下，下一次允许请求时间为上次时间加上单个请求时间
			newTimeOfNextPermissionIssue = timeOfNextPermissionIssue + int64(t.perRequest)
		}

		// 尝试原子更新状态，成功则跳出循环
		if atomic.CompareAndSwapInt64(&t.state, timeOfNextPermissionIssue, newTimeOfNextPermissionIssue) {
			break
		}
	}

	// 计算需要睡眠的时间
	sleepDuration := time.Duration(newTimeOfNextPermissionIssue - now)
	if sleepDuration > 0 {
		// 如果需要睡眠，则执行睡眠并返回下一次允许请求的时间
		t.clock.Sleep(sleepDuration)
		return time.Unix(0, newTimeOfNextPermissionIssue)
	}
	// 如果不需要睡眠，返回当前时间（与 atomicLimiter 行为一致）
	return time.Unix(0, now)
}
