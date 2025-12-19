package gctuner

import (
	"runtime"
	"sync/atomic"
)

// finalizerCallback 定义了在对象被GC时调用的回调函数类型
type finalizerCallback func()

// finalizer 是一个用于监控对象GC事件的结构体
type finalizer struct {
	ref      *finalizerRef     // 指向finalizerRef的指针，用于设置finalizer
	callback finalizerCallback // 每次GC都会被调用，因为每次GC都会将ref设置为nil，下次GC会回收为nil引用的数据
	stopped  int32             // 停止标志，原子操作访问
}

// stop 停止finalizer回调的执行
func (f *finalizer) stop() {
	atomic.StoreInt32(&f.stopped, 1)
}

// finalizerRef 是一个辅助结构体，用于与runtime.SetFinalizer配合使用
type finalizerRef struct {
	parent *finalizer // 指向父级finalizer
}

// finalizerHandler 是设置给runtime的finalizer处理函数
func finalizerHandler(ref *finalizerRef) {
	// 如果已停止，则不再调用回调
	if atomic.LoadInt32(&ref.parent.stopped) == 1 {
		return
	}
	// 调用用户定义的回调函数
	ref.parent.callback()
	// 重新设置finalizer，确保下次GC时仍能触发
	runtime.SetFinalizer(ref, finalizerHandler)
}

// newFinalizer 创建并返回一个finalizer对象，调用者需要保存该对象以确保它不会被GC
// Go运行时保证每次GC时都会调用回调函数
func newFinalizer(callback finalizerCallback) *finalizer {
	f := &finalizer{
		callback: callback,
	}
	f.ref = &finalizerRef{
		parent: f,
	}
	// 设置finalizer处理函数
	runtime.SetFinalizer(f.ref, finalizerHandler)
	f.ref = nil // 触发GC
	return f
}
