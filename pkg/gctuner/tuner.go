// tuner.go 实现了Go程序的动态GC调优功能

package gctuner

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"sync/atomic"

	"github.com/docker/go-units"
	"github.com/shirou/gopsutil/mem"
)

// GC百分比的默认值、最大值和最小值配置
var (
	maxGCPercent     uint32 = 500 // 最大GC百分比
	minGCPercent     uint32 = 50  // 最小GC百分比
	defaultGCPercent uint32 = 100 // 默认GC百分比
)

// init 初始化函数，从环境变量GOGC中读取默认GC百分比
func init() {
	gogcEnv := os.Getenv("GOGC")
	gogc, err := strconv.ParseInt(gogcEnv, 10, 32)
	if err != nil {
		return
	}
	if gogc > 0 {
		defaultGCPercent = uint32(gogc)
	}
}

// Tuning 设置GC调优的内存阈值
// 当进行调优时，环境变量GOGC将不再生效
// threshold: 如果为0则禁用调优
func Tuning(threshold uint64) {
	// 如果阈值为0且已存在全局调优器，则停止调优
	if threshold <= 0 && globalTuner != nil {
		globalTuner.stop()
		globalTuner = nil
		return
	}
	// 如果不存在全局调优器，则创建新的
	if globalTuner == nil {
		globalTuner = newTuner(threshold)
		return
	}
	// 更新阈值
	globalTuner.setThreshold(threshold)
}

// GetGcPercent 获取当前的GC百分比
func GetGcPercent() uint32 {
	if globalTuner == nil {
		return defaultGCPercent
	}

	return globalTuner.getGCPercent()
}

// GetMaxGCPercent 获取最大GC百分比值
func GetMaxGCPercent() uint32 {
	return atomic.LoadUint32(&maxGCPercent)
}

// SetMaxGCPercent 设置新的最大GC百分比值
func SetMaxGCPercent(percent uint32) uint32 {
	return atomic.SwapUint32(&maxGCPercent, percent)
}

// GetMinGCPercent 获取最小GC百分比值
func GetMinGCPercent() uint32 {
	return atomic.LoadUint32(&minGCPercent)
}

// SetMinGCPercent 设置新的最小GC百分比值
func SetMinGCPercent(percent uint32) uint32 {
	return atomic.SwapUint32(&minGCPercent, percent)
}

// 全局唯一的GC调优器实例
var globalTuner *tuner = nil

/*
内存堆结构示意图:
_______________  => limit: 主机/cgroup内存硬限制
|               |
|---------------| => threshold: 当gc_trigger < threshold时增加GCPercent
|               |
|---------------| => gc_trigger: heap_live + heap_live * GCPercent / 100
|               |
|---------------|
|   heap_live   |
|_______________|

Go运行时只在达到gc_trigger时触发GC，gc_trigger受GCPercent和heap_live影响。
因此我们可以动态改变GCPercent来调优GC性能。
*/

// tuner GC调优器结构体
type tuner struct {
	finalizer *finalizer // finalizer对象，用于监控GC事件
	gcPercent uint32     // 当前GC百分比
	threshold uint64     // 高水位线阈值，单位字节
}

// tuning 检查内存使用情况并动态调整GC百分比
// Go运行时保证此方法会被串行调用
func (t *tuner) tuning() {
	inuse := readMemoryInuse()    // 获取当前使用的内存量
	threshold := t.getThreshold() // 获取阈值
	// 如果阈值小于等于0，停止GC调优
	if threshold <= 0 {
		return
	}
	// 计算并设置新的GC百分比
	t.setGCPercent(calcGCPercent(inuse, threshold))
	return
}

// calcGCPercent 根据当前内存使用量和阈值计算GC百分比
// threshold = inuse + inuse * (gcPercent / 100)
// => gcPercent = (threshold - inuse) / inuse * 100
// 如果 threshold < inuse * 2, 则 gcPercent < 100, GC更积极以避免OOM
// 如果 threshold > inuse * 2, 则 gcPercent > 100, GC更保守以避免频繁GC
func calcGCPercent(inuse, threshold uint64) uint32 {
	// 参数无效
	if inuse == 0 || threshold == 0 {
		return defaultGCPercent
	}
	// 使用中的堆内存大于阈值，使用最小百分比
	if threshold <= inuse {
		return minGCPercent
	}

	// 计算GC百分比
	gcPercent := uint32(math.Floor(float64(threshold-inuse) / float64(inuse) * 100))
	if gcPercent < minGCPercent {
		return minGCPercent
	} else if gcPercent > maxGCPercent {
		return maxGCPercent
	}

	return gcPercent
}

// newTuner 创建新的调优器实例
func newTuner(threshold uint64) *tuner {
	t := &tuner{
		gcPercent: defaultGCPercent,
		threshold: threshold,
	}
	// 设置finalizer来监控GC事件
	t.finalizer = newFinalizer(t.tuning)
	return t
}

// stop 停止调优器
func (t *tuner) stop() {
	t.finalizer.stop()
}

// setThreshold 设置阈值
func (t *tuner) setThreshold(threshold uint64) {
	atomic.StoreUint64(&t.threshold, threshold)
}

// getThreshold 获取阈值
func (t *tuner) getThreshold() uint64 {
	return atomic.LoadUint64(&t.threshold)
}

// setGCPercent 设置GC百分比
func (t *tuner) setGCPercent(percent uint32) uint32 {
	atomic.StoreUint32(&t.gcPercent, percent)
	// 调用runtime接口设置GC百分比
	return uint32(debug.SetGCPercent(int(percent)))
}

// getGCPercent 获取GC百分比
func (t *tuner) getGCPercent() uint32 {
	return atomic.LoadUint32(&t.gcPercent)
}

// TuningWithFromHuman 使用人类可读的字符串格式设置阈值
// 例如: "b/B", "k/K" "kb/Kb" "mb/Mb", "gb/Gb" "tb/Tb" "pb/Pb"
func TuningWithFromHuman(threshold string) {
	parseThreshold, err := units.FromHumanSize(threshold)
	if err != nil {
		fmt.Println("parse threshold error:", err)
		return
	}
	Tuning(uint64(parseThreshold))
}

// TuningWithAuto 通过自动计算总内存量来设置阈值
func TuningWithAuto(isContainer bool) {
	var (
		threshold uint64
		err       error
	)
	// 根据是否为容器环境选择不同的内存限制获取方式
	if isContainer {
		threshold, err = getCGroupMemoryLimit()
	} else {
		threshold, err = getNormalMemoryLimit()
	}
	if err != nil {
		fmt.Println("get memory limit error:", err)
		return
	}
	// 使用70%的内存限制作为阈值
	Tuning(uint64(float64(threshold) * 0.7))
}

// cgroup内存限制文件路径
const cgroupMemLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

// getCGroupMemoryLimit 获取cgroup内存限制
func getCGroupMemoryLimit() (uint64, error) {
	usage, err := readUint(cgroupMemLimitPath)
	if err != nil {
		return 0, err
	}
	machineMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	// 取cgroup限制和机器总内存中的较小值
	limit := uint64(math.Min(float64(usage), float64(machineMemory.Total)))

	return limit, nil
}

// getNormalMemoryLimit 获取普通环境下的内存限制
func getNormalMemoryLimit() (uint64, error) {
	machineMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return machineMemory.Total, nil
}

// parseUint 解析无符号整数，处理负数值的情况
// 复制自 https://github.com/containerd/cgroups/blob/318312a373405e5e91134d8063d04d59768a1bff/utils.go#L251
func parseUint(s string, base, bitSize int) (uint64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. 处理大于MinInt64的负值
		// 2. 处理小于MinInt64的负值
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil && errors.Is(intErr.(*strconv.NumError).Err, strconv.ErrRange) && intValue < 0 {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// readUint 从文件中读取无符号整数
func readUint(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return parseUint(string(bytes.TrimSpace(data)), 10, 64)
}
