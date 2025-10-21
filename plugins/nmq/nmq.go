package nmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/nmq/interfaces"
	"github.com/nmq/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	cfg     *Config
}

// NewNmq 创建一个组件管理器
func NewNmq(op ...Option) *Nmq {
	nmq := &Nmq{
		cfg: DefaultConfig(),
	}
	for _, opt := range op {
		opt.apply(nmq)
	}

	nmq.components = make(map[string]interfaces.Component)

	return nmq
}

// GetComponent 获取组件
func (nmq *Nmq) GetComponent(uuid string) interfaces.Component {
	nmq.mux.RLock()
	defer nmq.mux.RUnlock()
	return nmq.components[uuid]
}

// AddCommand 添加命令
func (nmq *Nmq) AddCommand(cmds ...*cobra.Command) {
	nmq.rootCmd.AddCommand(cmds...)
}

// WgAdd 添加协程
func (nmq *Nmq) WgAdd(delta int) {
	nmq.wg.Add(delta)
}

// WaitGroup 等待所有协程完成
func (nmq *Nmq) WaitGroup() {
	nmq.wg.Wait()
}

// GetComponentManager 获取组件管理器
func (nmq *Nmq) GetComponentManager() interfaces.ComponentManager {
	return nmq
}

// GetInterface 获取接口
func (nmq *Nmq) GetInterface(uuid string) any {
	for _, component := range nmq.components {
		f := component.GetInterface(uuid)
		if f != nil {
			return f
		}
	}
	return nil
}

// Init 初始化组件
func (nmq *Nmq) Init() error {
	// 将自身注册进组件
	nmq.RegisterComponent(nmq.GetName(), nmq)
	// 没有指定日志记录器的情况下，创建默认日志记录器
	if nmq.logger == nil {
		log, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
			utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
			utils.SetLogMaxAge(30), utils.SetLogCompress(true),
			utils.SetLogFilename("./log/nmq.log"), utils.SetLogLevelKey("info"))
		if err != nil {
			fmt.Println("Failed to create logger")
			return err
		}
		nmq.logger = log
	}

	if nmq.ctx == nil {
		ncpContext, cancel := context.WithCancel(context.Background())
		nmq.ctx = ncpContext
		nmq.cancel = cancel
	}

	if nmq.rootCmd == nil {
		nmq.rootCmd = &cobra.Command{
			Use:   "nmq",
			Short: "nmq is a component manager",
			Run: func(cmd *cobra.Command, args []string) {
				err := cmd.Help()
				if err != nil {
					nmq.logger.Error("Failed to create logger", zap.Error(err))
					return
				}
			},
		}
	}

	// config --config.file data
	nmq.rootCmd.PersistentFlags().String("config.file", "nmq.yaml", "input the config file name")
	// Bind viper to the root command
	err := viper.BindPFlag("configFile", nmq.rootCmd.PersistentFlags().Lookup("config.file"))
	if err != nil {
		nmq.logger.Error("Error binding flag", zap.Error(err))
		return err
	}
	viper.SetConfigType("yaml")

	for _, component := range nmq.components {
		// 自己不能初始化自己
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Init()
		if err != nil {
			nmq.logger.Error("Failed to init component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Start 启动组件
func (nmq *Nmq) Start() error {
	// 加载ncp各种辅助代理
	err := loadAgentByConfig(nmq.cfg)
	if err != nil {
		return err
	}

	for _, component := range nmq.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Start()
		if err != nil {
			nmq.logger.Error("Failed to start component", zap.Error(err))
			return err
		}
	}
	return nil
}

// Stop 停止组件
func (nmq *Nmq) Stop() error {

	nmq.cancel()
	for _, component := range nmq.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Stop()
		if err != nil {
			nmq.logger.Error("Failed to stop component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Reset 重置组件
func (nmq *Nmq) Reset() error {
	for _, component := range nmq.components {
		if component.GetName() == "nmq" {
			continue
		}
		err := component.Reset()
		if err != nil {
			nmq.logger.Error("Failed to reset component", zap.Error(err))
			return err
		}
	}
	return nil
}

// GetName 获取组件名称
func (nmq *Nmq) GetName() string {
	return "nmq"
}

// GetVersion 获取组件版本
func (nmq *Nmq) GetVersion() string {
	return "v1.0.0.0"
}

// Notify 通知组件
func (nmq *Nmq) Notify(event string, data any) {
	for _, component := range nmq.components {
		component.Notify(event, data)
	}
}

// GetStatus 获取组件状态
func (nmq *Nmq) GetStatus() interfaces.ComponentStatus {
	return nmq.status
}

// GetContext 获取上下文
func (nmq *Nmq) GetContext() context.Context {
	return nmq.ctx
}

func (nmq *Nmq) GetCancel() context.CancelFunc {
	return nmq.cancel
}

// GetLogger 获取日志记录器
func (nmq *Nmq) GetLogger() *zap.Logger {
	return nmq.logger
}

// Execute 运行组件
func (nmq *Nmq) Execute() error {
	err := nmq.Init()
	if err != nil {
		fmt.Printf("Failed to init nmq, err: %s", err.Error())
		return err
	}

	nmq.logger.Info("Starting nmq")
	err = nmq.Start()
	if err != nil {
		nmq.logger.Error("Failed to start nmq", zap.Error(err))
		return err
	}
	defer nmq.logger.Info("Waiting for nmq to exit")
	if err = nmq.rootCmd.Execute(); err != nil {
		nmq.logger.Error("Failed to execute nmq", zap.Error(err))
		return err
	}
	// 停止所有协程
	err = nmq.Stop()
	if err != nil {
		nmq.logger.Error("Failed to stop nmq", zap.Error(err))
		return err
	}
	// 清理资源
	err = nmq.Reset()
	if err != nil {
		nmq.logger.Error("Failed to reset nmq", zap.Error(err))
		return err
	}
	nmq.logger.Info("Exit nmq")
	return nil
}

// RegisterComponent 注册组件
func (nmq *Nmq) RegisterComponent(componentName string, component interfaces.Component) {
	nmq.mux.Lock()
	defer nmq.mux.Unlock()
	nmq.components[componentName] = component
}
