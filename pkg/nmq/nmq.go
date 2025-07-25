package nmq

import (
	"context"
	"fmt"
	"github.com/nmq/interfaces"
	"github.com/nmq/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
)

// Nmq 组件管理器
type Nmq struct {
	status     interfaces.ComponentStatus
	mux        sync.RWMutex                    // for components
	components map[string]interfaces.Component // component name to component

	logger  *zap.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	rootCmd *cobra.Command
	wg      sync.WaitGroup // 协程同步
}

// An Option configures a Logger.
type Option interface {
	apply(*Nmq)
}

// optionFunc wraps a func so it satisfies the Option interface.
type optionFunc func(*Nmq)

// apply applies the Option to the given Nmq.
func (f optionFunc) apply(n *Nmq) {
	f(n)
}

// SetContext 设置日志文件名
func SetContext(ctx context.Context) Option {
	return optionFunc(func(n *Nmq) {
		n.ctx = ctx
	})
}

// SetCancel 设置取消函数
func SetCancel(cancel context.CancelFunc) Option {
	return optionFunc(func(n *Nmq) {
		n.cancel = cancel
	})
}

// SetLogger 设置日志记录器
func SetLogger(logger *zap.Logger) Option {
	return optionFunc(func(n *Nmq) {
		n.logger = logger
	})
}

// NewNcp 创建一个组件管理器
func NewNcp(op ...Option) *Nmq {
	ncp := &Nmq{}
	for _, opt := range op {
		opt.apply(ncp)
	}

	err := ncp.Init(nil)
	if err != nil {
		return nil
	}

	ncp.wg.Add(1)
	return ncp
}

// GetComponent 获取组件
func (ncp *Nmq) GetComponent(uuid string) interfaces.Component {
	ncp.mux.RLock()
	defer ncp.mux.RUnlock()
	return ncp.components[uuid]
}

// AddCommand 添加命令
func (ncp *Nmq) AddCommand(cmds ...*cobra.Command) {
	ncp.rootCmd.AddCommand(cmds...)
}

// WgAdd 添加协程
func (ncp *Nmq) WgAdd(delta int) {
	ncp.wg.Add(delta)
}

// WaitGroup 等待所有协程完成
func (ncp *Nmq) WaitGroup() {
	ncp.wg.Wait()
}

// GetComponentManager 获取组件管理器
func (ncp *Nmq) GetComponentManager() interfaces.ComponentManager {
	return ncp
}

// GetInterface 获取接口
func (ncp *Nmq) GetInterface(uuid string) any {
	for _, component := range ncp.components {
		f := component.GetInterface(uuid)
		if f != nil {
			return f
		}
	}
	return nil
}

// Init 初始化组件
func (ncp *Nmq) Init(ctx interfaces.NmqContext) error {
	ncp.components = make(map[string]interfaces.Component)
	// 将自身注册进组件
	ncp.RegisterComponent(ncp.GetName(), ncp)
	// 没有指定日志记录器的情况下，创建默认日志记录器
	if ncp.logger == nil {
		log, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
			utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
			utils.SetLogMaxAge(30), utils.SetLogCompress(true),
			utils.SetLogFilename("./log/nmq.log"), utils.SetLogLevelKey("info"))
		if err != nil {
			fmt.Println("Failed to create logger")
			return err
		}
		ncp.logger = log
	}

	if ncp.ctx == nil {
		ncpContext, cancel := context.WithCancel(context.Background())
		ncp.ctx = ncpContext
		ncp.cancel = cancel
	}

	if ncp.rootCmd == nil {
		ncp.rootCmd = &cobra.Command{
			Use:   "nmq",
			Short: "NCP is a component manager",
			Run: func(cmd *cobra.Command, args []string) {
				err := cmd.Help()
				if err != nil {
					ncp.logger.Error("Failed to create logger", zap.Error(err))
					return
				}
			},
		}
	}

	// config --config.file data
	ncp.rootCmd.PersistentFlags().String("config.file", "nmq.yaml", "input the config file name")
	// Bind viper to the root command
	err := viper.BindPFlag("configFile", ncp.rootCmd.PersistentFlags().Lookup("config.file"))
	if err != nil {
		ncp.logger.Error("Error binding flag", zap.Error(err))
		return err
	}
	viper.SetConfigType("yaml")

	for _, component := range ncp.components {
		// 自己不能初始化自己
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Init(ncp)
		if err != nil {
			ncp.logger.Error("Failed to init component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Start 启动组件
func (ncp *Nmq) Start() error {
	for _, component := range ncp.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Start()
		if err != nil {
			ncp.logger.Error("Failed to start component", zap.Error(err))
			return err
		}
	}
	return nil
}

// Stop 停止组件
func (ncp *Nmq) Stop() error {

	ncp.cancel()
	for _, component := range ncp.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Stop()
		if err != nil {
			ncp.logger.Error("Failed to stop component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Reset 重置组件
func (ncp *Nmq) Reset() error {
	for _, component := range ncp.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Reset()
		if err != nil {
			ncp.logger.Error("Failed to reset component", zap.Error(err))
			return err
		}
	}
	return nil
}

// GetName 获取组件名称
func (ncp *Nmq) GetName() string {
	return "nmq"
}

// GetVersion 获取组件版本
func (ncp *Nmq) GetVersion() string {
	return "v1.0.0.0"
}

// Notify 通知组件
func (ncp *Nmq) Notify(event string, data any) {
	for _, component := range ncp.components {
		component.Notify(event, data)
	}
}

// GetStatus 获取组件状态
func (ncp *Nmq) GetStatus() interfaces.ComponentStatus {
	return ncp.status
}

// GetContext 获取上下文
func (ncp *Nmq) GetContext() context.Context {
	return ncp.ctx
}

// GetLogger 获取日志记录器
func (ncp *Nmq) GetLogger() *zap.Logger {
	return ncp.logger
}

// Execute 运行组件
func (ncp *Nmq) Execute() error {

	ncp.logger.Info("Starting NCP")
	err := ncp.Start()
	if err != nil {
		ncp.logger.Error("Failed to start NCP", zap.Error(err))
		return err
	}
	defer ncp.logger.Info("Waiting for NCP to exit")
	if err = ncp.rootCmd.Execute(); err != nil {
		ncp.logger.Error("Failed to execute NCP", zap.Error(err))
		return err
	}
	// 停止所有协程
	err = ncp.Stop()
	if err != nil {
		ncp.logger.Error("Failed to stop NCP", zap.Error(err))
		return err
	}
	// 清理资源
	err = ncp.Reset()
	if err != nil {
		ncp.logger.Error("Failed to reset NCP", zap.Error(err))
		return err
	}
	ncp.logger.Info("Exit NCP")
	return nil
}

// RegisterComponent 注册组件
func (ncp *Nmq) RegisterComponent(componentName string, component interfaces.Component) {
	ncp.mux.Lock()
	defer ncp.mux.Unlock()
	ncp.components[componentName] = component
}
