package utils

import (
	"fmt"
	"sync/atomic"
)

type AtomicInt struct {
	val int64
}

func (a *AtomicInt) Add(delta int) {
	atomic.AddInt64(&a.val, int64(delta))
}

func (a *AtomicInt) Load() int {
	return int(atomic.LoadInt64(&a.val))
}

func (a *AtomicInt) Store(val int) {
	atomic.StoreInt64(&a.val, int64(val))
}

func (a *AtomicInt) String() string {
	return fmt.Sprintf("%d", a.val)
}

func NewAtomicInt(val int) *AtomicInt {
	return &AtomicInt{val: int64(val)}
}
func NewAtomicInt64(val int64) *AtomicInt {
	return &AtomicInt{val: val}
}
func (a *AtomicInt) CompareAndSwap(oldVal, newVal int) bool {
	return atomic.CompareAndSwapInt64(&a.val, int64(oldVal), int64(newVal))
}
func (a *AtomicInt) CompareAndSwap64(oldVal, newVal int64) bool {
	return atomic.CompareAndSwapInt64(&a.val, oldVal, newVal)
}
func (a *AtomicInt) Swap(newVal int) int {
	return int(atomic.SwapInt64(&a.val, int64(newVal)))
}
func (a *AtomicInt) Swap64(newVal int64) int64 {
	return atomic.SwapInt64(&a.val, newVal)
}
func (a *AtomicInt) Inc() int {
	return int(atomic.AddInt64(&a.val, 1))
}
func (a *AtomicInt) Dec() int {
	return int(atomic.AddInt64(&a.val, -1))
}
func (a *AtomicInt) Get() int {
	return int(atomic.LoadInt64(&a.val))
}
func (a *AtomicInt) Get64() int64 {
	return atomic.LoadInt64(&a.val)
}
func (a *AtomicInt) Set(val int) {
	atomic.StoreInt64(&a.val, int64(val))
}
func (a *AtomicInt) Set64(val int64) {
	atomic.StoreInt64(&a.val, val)
}
func (a *AtomicInt) Reset() {
	atomic.StoreInt64(&a.val, 0)
}
