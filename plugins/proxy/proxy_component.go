package proxy

import (
	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"github.com/spf13/cobra"
)

type Component struct {
	nmq.ComponentBase
}

// NewComponent 创建网络组件实例
func NewComponent(ctx nmq.Context) *Component {
	proxy := &Component{
		ComponentBase: nmq.NewComponentBase(ctx),
	}

	// 注册命令行
	proxyCommand := cobra.Command{
		Use:        "proxy",
		Short:      "Proxy commands",
		Long:       "Proxy commands",
		SuggestFor: []string{"proxy --config.file=proxy.yaml"},
		RunE:       proxy.Run,
	}

	proxy.ComponentManager.AddCommand(&proxyCommand)

	return proxy
}

// GetInterface 获取组件内部某个接口的实现
//
// @param uuid string 接口唯一标识
// @return any 接口实现对象或 nil
func (c *Component) GetInterface(uuid string) any {
	return nil
}

// Init 初始化组件
//
// @param ctx NmqContext 上下文环境
// @return error 错误信息
func (c *Component) Init() error {

	return nil
}

// Start 启动组件
//
// @return error 错误信息
func (c *Component) Start() error {
	return nil
}

// Stop 停止组件
//
// @return error 错误信息
func (c *Component) Stop() error {
	return nil
}

// Reset 重置组件
//
// @return error 错误信息
func (c *Component) Reset() error {
	return nil
}

// GetName 获取组件名称
//
// @return string 组件名称
func (c *Component) GetName() string {
	return interfaces.ProxyComponentName
}

// GetVersion 获取组件版本号
//
// @return string 版本号
func (c *Component) GetVersion() string {
	return "1.0.0"
}

// Notify 接收系统广播事件
//
// @param event string 事件名称
// @param data any 附加数据
func (c *Component) Notify(event string, data any) {
	return
}

// GetStatus 获取组件当前状态
//
// @return ComponentStatus 当前状态
func (c *Component) GetStatus() nmq.ComponentStatus {
	return nmq.ComponentOk
}

func (c *Component) Run(cmd *cobra.Command, args []string) error {

	return nil
}
