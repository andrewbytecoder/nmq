package utils

import (
	"encoding/binary"
	"unsafe"
)

// NativelyLittle 返回当前系统的字节序是否为 little-endian
func NativelyLittle() bool {
	var NativeEndian binary.ByteOrder
	// 通过检查 int16 的内存布局来确定系统字节序
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		NativeEndian = binary.BigEndian
	} else {
		NativeEndian = binary.LittleEndian
	}
	return NativeEndian == binary.LittleEndian
}
