// @title Component Management API
// @version 1.0
// @description 组件管理核心接口定义
// @Auth @wangyazhou

package nmq

import (
	"github.com/spf13/cobra"
)

type ComponentManager interface {
	GetComponent(name string) Component
	AddCommand(cmds ...*cobra.Command)
	WgAdd(delta int)
	WaitGroup()
}
