// @title Component Management API
// @version 1.0
// @description 组件管理核心接口定义
// @Auth @wangyazhou

package interfaces

import (
	"context"
	"go.uber.org/zap"
)

// NmqContext 是组件初始化时使用的上下文接口
//
// 提供对全局 Context、Logger 和组件管理器的访问
type NmqContext interface {
	GetContext() context.Context
	GetLogger() *zap.Logger
	GetComponentManager() ComponentManager
}

// ComponentStatus 表示组件的生命周期状态
//
// @Description 组件当前所处的状态，用于监控和调试
// @Description - ComponentOk: 正常运行
// @Description - ComponentInit: 初始化完成
// @Description - ComponentRunning: 运行中
// @Description - ComponentStopped: 已停止
// @Description - ComponentReset: 已重置
type ComponentStatus uint

const (
	ComponentOk      ComponentStatus = 0
	ComponentInit    ComponentStatus = 1
	ComponentRunning ComponentStatus = 2
	ComponentStopped ComponentStatus = 3
	ComponentReset   ComponentStatus = 4
)

// Component 是所有可注册组件必须实现的核心接口
//
// @Description 每个组件都需实现以下生命周期方法
type Component interface {
	// GetInterface 获取组件内部某个接口的实现
	//
	// @param uuid string 接口唯一标识
	// @return any 接口实现对象或 nil
	GetInterface(uuid string) any

	// Init 初始化组件
	//
	// @param ctx NmqContext 上下文环境
	// @return error 错误信息
	Init() error

	// Start 启动组件
	//
	// @return error 错误信息
	Start() error

	// Stop 停止组件
	//
	// @return error 错误信息
	Stop() error

	// Reset 重置组件
	//
	// @return error 错误信息
	Reset() error

	// GetName 获取组件名称
	//
	// @return string 组件名称
	GetName() string

	// GetVersion 获取组件版本号
	//
	// @return string 版本号
	GetVersion() string

	// Notify 接收系统广播事件
	//
	// @param event string 事件名称
	// @param data any 附加数据
	Notify(event string, data any)

	// GetStatus 获取组件当前状态
	//
	// @return ComponentStatus 当前状态
	GetStatus() ComponentStatus
}
