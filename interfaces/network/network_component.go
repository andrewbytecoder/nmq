package network

import "github.com/nmq/utils"

// interface uuid: network_snow_flake

// NetSnowFlake 创建一个雪花ID生成器
type NetSnowFlake interface {
	Generate() utils.SnowID
}
