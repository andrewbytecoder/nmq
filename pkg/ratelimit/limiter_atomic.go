package ratelimit

import (
	"time"

	"sync/atomic"
	"unsafe"
)

// state holds the state of the atomic limiter
type state struct {
	last     time.Time     // 上次请求的时间戳
	sleepFor time.Duration // 需要睡眠的时间长度
}

// atomicLimiter 是基于原子操作实现的限流器
type atomicLimiter struct {
	state   unsafe.Pointer // 指向 state 结构的原子指针
	padding [56]byte       // 缓存行填充，避免伪共享(cache line size 64 - 指针大小 8 = 56)

	perRequest time.Duration // 每个请求之间的时间间隔 (per/rate)
	// 最大补偿个数，用于补偿请求间隔
	maxSlack time.Duration // 最大松弛时间，等于 slack * perRequest
	clock    Clock         // 时钟接口，用于获取当前时间和睡眠
}

// newAtomicBased 创建一个新的基于原子操作的限流器
func newAtomicBased(rate int, opts ...Option) *atomicLimiter {
	// 构建配置参数
	c := buildConfig(opts)
	// 计算每个请求应该间隔的时间
	perRequest := c.per / time.Duration(rate)
	l := &atomicLimiter{
		perRequest: perRequest,
		maxSlack:   -1 * time.Duration(c.slack) * perRequest, // 最大松弛时间为 slack 倍的单个请求时间
		clock:      c.clock,
	}

	// 初始化状态
	initialState := state{
		last:     time.Time{}, // 初始时间为零值
		sleepFor: 0,           // 初始不需要睡眠
	}
	// 原子存储初始状态
	atomic.StorePointer(&l.state, unsafe.Pointer(&initialState))
	return l
}

// Take 阻塞以确保多次调用 Take 之间的平均时间符合 per/rate 的限制
func (t *atomicLimiter) Take() time.Time {
	var (
		newState state
		taken    bool
		interval time.Duration
	)
	// 使用 CAS 循环直到成功更新状态
	for !taken {
		// 获取当前时间
		now := t.clock.Now()

		// 原子加载当前状态
		previousStatePointer := atomic.LoadPointer(&t.state)
		oldState := (*state)(previousStatePointer)

		// 初始化新状态
		newState = state{
			last:     now,               // 设置当前时间为最新请求时间
			sleepFor: oldState.sleepFor, // 继承之前的睡眠时间
		}

		// 如果是第一次请求，则直接允许
		if oldState.last.IsZero() {
			taken = atomic.CompareAndSwapPointer(&t.state, previousStatePointer, unsafe.Pointer(&newState))
			continue
		}

		// 计算需要睡眠的时间：基于每个请求的预算时间和上次请求到现在的时间差
		// 因为请求可能比预算时间长，这个数值可能是负数，并且会在请求间累加
		newState.sleepFor += t.perRequest - now.Sub(oldState.last)

		// 不应让 sleepFor 变得太消极，因为这意味着服务在短时间内大幅减速后会获得更高的 RPS
		// 有一次请求间隔超长之后会补偿slack个个数
		// 1. 上次请求间隔时间超过 perRequest 会导致sleepFor < 0，但是如果不是一次超过 slack次就不会触发最大补偿个数修正
		// 比如超时时间间隔是两次，那么后面会每次都修正sleepFor 直到sleepFor > 0为止
		//2. 如果sleepFor < t.maxSlack 会触发最大补偿个数修正，最多允许补偿slack个数
		if newState.sleepFor < t.maxSlack {
			newState.sleepFor = t.maxSlack
		}

		// 如果需要睡眠，则调整最后时间和间隔
		if newState.sleepFor > 0 {
			newState.last = newState.last.Add(newState.sleepFor)
			// 本次需要睡眠将sleepFor重置为0,
			interval, newState.sleepFor = newState.sleepFor, 0
		} // else 如果sleepFor <= 0,则不需要睡眠, 继续下一次循环，直接一直请求 slack次数

		// 尝试原子更新状态
		taken = atomic.CompareAndSwapPointer(&t.state, previousStatePointer, unsafe.Pointer(&newState))
	}

	// 执行实际的睡眠
	t.clock.Sleep(interval)
	// 返回最后一次请求的时间
	return newState.last
}
