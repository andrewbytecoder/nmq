package hash

import (
	"encoding/binary"
)

// Hash 计算给定数据的哈希值。
//
// @Summary 计算给定数据的哈希值
// @Description 此函数使用类似 Murmur 哈希的算法，根据给定的数据和种子值计算哈希值。
// @Tags Hash
// @Success 200 {uint32} uint32 "成功计算出的哈希值"
// @Router /hash [post]
func Hash(data []byte, seed uint32) uint32 {
	// Similar to murmur hash
	const (
		m = uint32(0xc6a4a793)
		r = uint32(24)
	)
	var (
		h = seed ^ (uint32(len(data)) * m)
		i int
	)

	for n := len(data) - len(data)%4; i < n; i += 4 {
		h += binary.LittleEndian.Uint32(data[i:])
		h *= m
		h ^= (h >> 16)
	}

	switch len(data) - i {
	default:
		panic("not reached")
	case 3:
		h += uint32(data[i+2]) << 16
		fallthrough
	case 2:
		h += uint32(data[i+1]) << 8
		fallthrough
	case 1:
		h += uint32(data[i])
		h *= m
		h ^= (h >> r)
	case 0:
	}

	return h
}
