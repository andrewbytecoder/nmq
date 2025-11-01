package mq

import (
	"github.com/andrewbytecoder/nmq/interfaces"
	"go.uber.org/zap"
)

type MessageQueueComponent struct {
	ctx interfaces.NmqContext
	log *zap.Logger
}

// NewNetComponent 创建网络组件实例
func NewNetComponent(ctx interfaces.NmqContext) *MessageQueueComponent {
	return &MessageQueueComponent{
		ctx: ctx,
		log: ctx.GetLogger(),
	}
}

// GetInterface 获取组件内部某个接口的实现
//
// @param uuid string 接口唯一标识
// @return any 接口实现对象或 nil
func (nc *MessageQueueComponent) GetInterface(uuid string) any {
	return nil
}

// Init 初始化组件
//
// @param ctx NmqContext 上下文环境
// @return error 错误信息
func (nc *MessageQueueComponent) Init() error {

	return nil
}

// Start 启动组件
//
// @return error 错误信息
func (nc *MessageQueueComponent) Start() error {
	return nil
}

// Stop 停止组件
//
// @return error 错误信息
func (nc *MessageQueueComponent) Stop() error {
	return nil
}

// Reset 重置组件
//
// @return error 错误信息
func (nc *MessageQueueComponent) Reset() error {
	return nil
}

// GetName 获取组件名称
//
// @return string 组件名称
func (nc *MessageQueueComponent) GetName() string {
	return "subscribe_component"
}

// GetVersion 获取组件版本号
//
// @return string 版本号
func (nc *MessageQueueComponent) GetVersion() string {
	return "1.0.0"
}

// Notify 接收系统广播事件
//
// @param event string 事件名称
// @param data any 附加数据
func (nc *MessageQueueComponent) Notify(event string, data any) {
	return
}

// GetStatus 获取组件当前状态
//
// @return ComponentStatus 当前状态
func (nc *MessageQueueComponent) GetStatus() interfaces.ComponentStatus {
	return interfaces.ComponentOk
}
