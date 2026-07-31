package proxy

import (
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"go.uber.org/zap"
)

type Component struct {
	ctx nmq.NmqContext
	log *zap.Logger
}

// NewComponent 创建网络组件实例
func NewComponent(ctx nmq.NmqContext) *Component {
	return &Component{
		ctx: ctx,
		log: ctx.GetLogger(),
	}
}

// GetInterface 获取组件内部某个接口的实现
//
// @param uuid string 接口唯一标识
// @return any 接口实现对象或 nil
func (nc *Component) GetInterface(uuid string) any {
	return nil
}

// Init 初始化组件
//
// @param ctx NmqContext 上下文环境
// @return error 错误信息
func (nc *Component) Init() error {

	return nil
}

// Start 启动组件
//
// @return error 错误信息
func (nc *Component) Start() error {
	return nil
}

// Stop 停止组件
//
// @return error 错误信息
func (nc *Component) Stop() error {
	return nil
}

// Reset 重置组件
//
// @return error 错误信息
func (nc *Component) Reset() error {
	return nil
}

// GetName 获取组件名称
//
// @return string 组件名称
func (nc *Component) GetName() string {
	return "subscribe_component"
}

// GetVersion 获取组件版本号
//
// @return string 版本号
func (nc *Component) GetVersion() string {
	return "1.0.0"
}

// Notify 接收系统广播事件
//
// @param event string 事件名称
// @param data any 附加数据
func (nc *Component) Notify(event string, data any) {
	return
}

// GetStatus 获取组件当前状态
//
// @return ComponentStatus 当前状态
func (nc *Component) GetStatus() nmq.ComponentStatus {
	return nmq.ComponentOk
}
