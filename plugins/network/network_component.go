package network

import (
	"hash/fnv"

	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/pkg/httpclient"
	"github.com/andrewbytecoder/nmq/pkg/utils"
	"go.uber.org/zap"
)

type NetComponent struct {
	ctx        interfaces.NmqContext
	log        *zap.Logger
	httpClient *httpclient.HttpClient
	snowNode   *utils.SnowNode
}

// NewNetComponent 创建网络组件实例
func NewNetComponent(ctx interfaces.NmqContext) *NetComponent {
	return &NetComponent{
		ctx: ctx,
		log: ctx.GetLogger(),
	}
}

// GetInterface 获取组件内部某个接口的实现
//
// @param uuid string 接口唯一标识
// @return any 接口实现对象或 nil
func (nc *NetComponent) GetInterface(uuid string) any {
	if uuid == "network_snow_flake" {
		return nc.snowNode
	}

	return nil
}

// Init 初始化组件
//
// @param ctx NmqContext 上下文环境
// @return error 错误信息
func (nc *NetComponent) Init() error {
	h := fnv.New64()
	_, err := h.Write([]byte(nc.GetName()))
	if err != nil {
		nc.log.Error("hash error", zap.Error(err))
		return err
	}

	nc.snowNode, err = utils.NewSnowNode(int64(h.Sum64()))
	if err != nil {
		nc.log.Error("snow node error", zap.Error(err))
		return err
	}

	return nil
}

// Start 启动组件
//
// @return error 错误信息
func (nc *NetComponent) Start() error {
	return nil
}

// Stop 停止组件
//
// @return error 错误信息
func (nc *NetComponent) Stop() error {
	return nil
}

// Reset 重置组件
//
// @return error 错误信息
func (nc *NetComponent) Reset() error {
	return nil
}

// GetName 获取组件名称
//
// @return string 组件名称
func (nc *NetComponent) GetName() string {
	return interfaces.NetworkComponentName
}

// GetVersion 获取组件版本号
//
// @return string 版本号
func (nc *NetComponent) GetVersion() string {
	return "1.0.0"
}

// Notify 接收系统广播事件
//
// @param event string 事件名称
// @param data any 附加数据
func (nc *NetComponent) Notify(event string, data any) {
	return
}

// GetStatus 获取组件当前状态
//
// @return ComponentStatus 当前状态
func (nc *NetComponent) GetStatus() interfaces.ComponentStatus {
	return interfaces.ComponentOk
}
