package nmq

import (
	"context"
	"fmt"

	"sync"

	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"github.com/andrewbytecoder/nmq/pkg/utils"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Nmq 组件管理器
type Nmq struct {
	status     nmq.ComponentStatus
	mux        sync.RWMutex             // for components
	components map[string]nmq.Component // component name to component

	logger  *zap.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	rootCmd *cobra.Command
	wg      sync.WaitGroup // 协程同步
	cfg     *Config

	pool *ants.Pool
}

// NewNmq 创建一个组件管理器
func NewNmq(op ...Option) *Nmq {
	n := &Nmq{
		cfg: DefaultConfig(),
	}
	for _, opt := range op {
		opt.apply(n)
	}

	n.components = make(map[string]nmq.Component)
	// 没有指定日志记录器的情况下，创建默认日志记录器
	if n.logger == nil {
		log, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
			utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
			utils.SetLogMaxAge(30), utils.SetLogCompress(true),
			utils.SetLogFilename("./log/ncp.log"), utils.SetLogLevelKey("info"))
		if err != nil {
			fmt.Println("Failed to create logger")
			return nil
		}
		n.logger = log
	}

	if n.ctx == nil {
		ncpContext, cancel := context.WithCancel(context.Background())
		n.ctx = ncpContext
		n.cancel = cancel
	}

	if n.rootCmd == nil {
		n.rootCmd = &cobra.Command{
			Use:   "nmp",
			Short: "NMP is a component manager",
			Run: func(cmd *cobra.Command, args []string) {
				err := cmd.Help()
				if err != nil {
					n.logger.Error("Failed to create logger", zap.Error(err))
					return
				}
			},
		}
	}

	// PersistentPreRunE: 命令在运行之前执行，并且子命令里面也会执行
	n.rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// parse flags
		err := n.Init()
		if err != nil {
			return err
		}
		n.logger.Info("Starting NCP")

		err = n.Start()
		if err != nil {
			n.logger.Error("Failed to start NCP", zap.Error(err))
			return err
		}
		return nil
	}

	// 运行结束之后执行
	n.rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		err := n.Stop()
		if err != nil {
			n.logger.Error("Failed to stop NCP", zap.Error(err))
			return err
		}
		// 在清理资源之前进行善后工作
		n.pool.Release()

		// todo: 清理资源，根据实际看是否需要将该部分动作放到Execute() 执行结束之后执行
		// 如果用户将部分自定义资源绑定到cobra中这里释放资源可能会有问题
		err = n.Reset()
		if err != nil {
			n.logger.Error("Failed to reset NCP", zap.Error(err))
			return err
		}
		n.logger.Info("Exit NCP")
		return nil
	}

	n.rootCmd.SetUsageFunc(usageFunc)
	// Make help just show the usage
	n.rootCmd.SetHelpTemplate(`{{.UsageString}}`)
	// config --config.file 无论在子命令还是主命令里面都只能使用一次
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.configFile, "config.file", "f", "ncp.yaml", "input the config file name")
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.certPath, "cert.path", "c", "./", "cert path for https")
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.workDir, "work", "w", "", "config the work path")

	return n
}

// usageFunc 自定义使用说明函数
// @Description 自定义使用说明函数
// @Return error 返回错误信息
func usageFunc(c *cobra.Command) error {
	// 你可以完全自由地定义输出内容
	_, _ = fmt.Fprintf(c.OutOrStderr(), "Usage: %s [command] [flags]\n\n", c.Name())
	_, _ = fmt.Fprintf(c.OutOrStderr(), "Available Commands:\n")

	for _, cmd := range c.Commands() {
		_, _ = fmt.Fprintf(c.OutOrStderr(), "  %s\t%s\n", cmd.Use, cmd.Short)
	}

	_, _ = fmt.Fprintf(c.OutOrStderr(), "\nFlags:\n")
	// 这里可以遍历 Flags 并格式化输出，或者直接调用 c.Flags().PrintDefaults()
	c.LocalFlags().PrintDefaults() // 利用库的默认打印功能

	_, _ = fmt.Fprintf(c.OutOrStderr(), "\nUse \"%s [command] --help\" for more information about a command.\n", c.Name())

	// 假设没有错误
	return nil
}

// GetComponent 获取组件
func (nmq *Nmq) GetComponent(uuid string) nmq.Component {
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
func (nmq *Nmq) GetComponentManager() nmq.ComponentManager {
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

	// 启动协程池
	nmq.pool, err = ants.NewPool(1000, ants.WithPanicHandler(func(err interface{}) {
		nmq.logger.Error("panic", zap.Any("panic", err))
	}))
	if err != nil {
		nmq.logger.Error("Failed to create pool", zap.Error(err))
		return err
	}

	for _, component := range nmq.components {
		if component.GetName() == nmq.GetName() {
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
		if component.GetName() == nmq.GetName() {
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
		if component.GetName() == nmq.GetName() {
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
	return interfaces.NmqComponentName
}

// GetVersion 获取组件版本
func (nmq *Nmq) GetVersion() string {
	return "v1.0.0.0"
}

// Notify 通知组件
func (nmq *Nmq) Notify(event string, data any) {
	for _, component := range nmq.components {
		if component.GetName() == nmq.GetName() {
			continue
		}
		component.Notify(event, data)
	}
}

func (nmq *Nmq) Submit(task func()) error {
	nmq.mux.Lock()
	defer nmq.mux.Unlock()
	err := nmq.pool.Submit(task)
	if err != nil {
		return err
	}
	return nil
}

func (nmq *Nmq) GetConfigFile() string {
	return nmq.cfg.configFile
}

func (nmq *Nmq) GetCertPath() string {
	return nmq.cfg.certPath
}

func (nmq *Nmq) GetWorkDir() string {
	return nmq.cfg.workDir
}

// GetStatus 获取组件状态
func (nmq *Nmq) GetStatus() nmq.ComponentStatus {
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

	nmq.logger.Info("Waiting for NCP to exit")
	if err := nmq.rootCmd.Execute(); err != nil {
		nmq.logger.Error("Failed to execute NCP", zap.Error(err))
		return err
	}

	return nil
}

// RegisterComponent 注册组件
func (nmq *Nmq) RegisterComponent(componentName string, component nmq.Component) {
	nmq.mux.Lock()
	defer nmq.mux.Unlock()
	nmq.components[componentName] = component
}
