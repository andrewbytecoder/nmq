package utils

import (
	"encoding/binary"
	"unsafe"
)

// NativeEndian 是当前系统的字节序
var NativeEndian binary.ByteOrder

func init() {
	// 通过检查 int16 的内存布局来确定系统字节序
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		NativeEndian = binary.BigEndian
	} else {
		NativeEndian = binary.LittleEndian
	}
}

// NativelyLittle 返回当前系统的字节序是否为 little-endian
func NativelyLittle() bool {
	return NativeEndian == binary.LittleEndian
}
