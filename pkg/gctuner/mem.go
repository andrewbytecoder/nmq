// mem.go 文件提供了内存使用情况的读取功能

package gctuner

import "runtime"

// memStats 用于存储Go运行时的内存统计信息
var memStats runtime.MemStats

// readMemoryInuse 读取当前程序已分配的内存量
// 返回值单位为字节(B)，表示当前正在使用的内存大小
func readMemoryInuse() uint64 {
	// 从运行时读取最新的内存统计信息到memStats变量中
	runtime.ReadMemStats(&memStats)
	// 返回已分配的内存量(Alloc字段)
	return memStats.Alloc
}
