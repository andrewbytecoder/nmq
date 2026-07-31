package nmq

import (
	"context"
	"fmt"
	"net/http"

	"sync"

	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"github.com/andrewbytecoder/nmq/pkg/utils"
	"github.com/andrewbytecoder/nmq/plugins/mq"
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

	logger      *zap.Logger
	atomicLevel zap.AtomicLevel // 日志原子级别，支持运行时动态修改
	debugServer *http.Server    // 调试 HTTP 服务
	ctx         context.Context
	cancel      context.CancelFunc
	rootCmd     *cobra.Command
	wg          sync.WaitGroup // 协程同步
	cfg         *Config

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
		log, atomicLevel, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
			utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
			utils.SetLogMaxAge(30), utils.SetLogCompress(true),
			utils.SetLogFilename("./log/ncp.log"), utils.SetLogLevelKey("info"))
		if err != nil {
			fmt.Println("Failed to create logger")
			return nil
		}
		n.logger = log
		n.atomicLevel = atomicLevel
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

	ParseFlags(n)

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

func (n *Nmq) SetUpMetricsHandler() (*metrics.Registry, error) {
	// 注册进程指标
	registerProcessMetrics := n.cfg.metricsConfig.enableMetrics && n.cfg.metricsConfig.registerProcessMetrics
	// 注册go指标
	registerGoMetrics := n.cfg.metricsConfig.enableMetrics && n.cfg.metricsConfig.registerGoMetrics
	// 注册服务指标
	registerServerMetrics := n.cfg.metricsConfig.enableMetrics && n.cfg.metricsConfig.registerServerMetrics
	registry := metrics.New(metrics.Config{
		Prefix:                 "ncp",
		RegisterProcessMetrics: registerProcessMetrics,
		RegisterGoMetrics:      registerGoMetrics,
		RegisterServerMetrics:  registerServerMetrics,
	})

	return registry, nil
}

// GetComponent 获取组件
func (n *Nmq) GetComponent(uuid string) nmq.Component {
	n.mux.RLock()
	defer n.mux.RUnlock()
	return n.components[uuid]
}

// AddCommand 添加命令
func (n *Nmq) AddCommand(cmds ...*cobra.Command) {
	n.rootCmd.AddCommand(cmds...)
}

// WgAdd 添加协程
func (n *Nmq) WgAdd(delta int) {
	n.wg.Add(delta)
}

// WaitGroup 等待所有协程完成
func (n *Nmq) WaitGroup() {
	n.wg.Wait()
}

// GetComponentManager 获取组件管理器
func (n *Nmq) GetComponentManager() nmq.ComponentManager {
	return n
}

// GetInterface 获取接口
func (n *Nmq) GetInterface(uuid string) any {
	for _, component := range n.components {
		if component.GetName() == n.GetName() {
			continue
		}
		f := component.GetInterface(uuid)
		if f != nil {
			return f
		}
	}
	return nil
}

func (n *Nmq) RegisterComponents() {
	// 注册组件
	messageQueueComponent := mq.NewNetComponent(n)
	n.components[messageQueueComponent.GetName()] = messageQueueComponent
}

// Init 初始化组件
func (n *Nmq) Init() error {
	// Bind viper to the root command
	err := viper.BindPFlag("configFile", n.rootCmd.PersistentFlags().Lookup("config.file"))
	if err != nil {
		n.logger.Error("Error binding flag", zap.Error(err))
		return err
	}
	viper.SetConfigType("yaml")

	for _, component := range n.components {
		// 自己不能初始化自己
		if component.GetName() == n.GetName() {
			continue
		}
		err := component.Init()
		if err != nil {
			n.logger.Error("Failed to init component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Start 启动组件
func (n *Nmq) Start() error {
	// 加载ncp各种辅助代理
	err := loadAgentByConfig(n.cfg)
	if err != nil {
		return err
	}

	// 启动调试 HTTP 服务（用于动态修改日志等级等）
	startDebugServer(n)

	// 启动协程池
	n.pool, err = ants.NewPool(1000, ants.WithPanicHandler(func(err interface{}) {
		n.logger.Error("panic", zap.Any("panic", err))
	}))
	if err != nil {
		n.logger.Error("Failed to create pool", zap.Error(err))
		return err
	}

	for _, component := range n.components {
		if component.GetName() == n.GetName() {
			continue
		}
		err := component.Start()
		if err != nil {
			n.logger.Error("Failed to start component", zap.Error(err))
			return err
		}
	}
	return nil
}

// Stop 停止组件
func (n *Nmq) Stop() error {

	n.cancel()

	// 停止调试 HTTP 服务
	stopDebugServer(n)

	for _, component := range n.components {
		if component.GetName() == n.GetName() {
			continue
		}
		err := component.Stop()
		if err != nil {
			n.logger.Error("Failed to stop component", zap.Error(err))
			return err
		}
	}

	return nil
}

// Reset 重置组件
func (n *Nmq) Reset() error {
	for _, component := range n.components {
		if component.GetName() == n.GetName() {
			continue
		}
		err := component.Reset()
		if err != nil {
			n.logger.Error("Failed to reset component", zap.Error(err))
			return err
		}
	}
	return nil
}

// GetName 获取组件名称
func (n *Nmq) GetName() string {
	return interfaces.NmqComponentName
}

// GetVersion 获取组件版本
func (n *Nmq) GetVersion() string {
	return "v1.0.0.0"
}

// Notify 通知组件
func (n *Nmq) Notify(event string, data any) {
	for _, component := range n.components {
		if component.GetName() == n.GetName() {
			continue
		}
		component.Notify(event, data)
	}
}

func (n *Nmq) Submit(task func()) error {
	n.mux.Lock()
	defer n.mux.Unlock()
	err := n.pool.Submit(task)
	if err != nil {
		return err
	}
	return nil
}

func (n *Nmq) GetConfigFile() string {
	return n.cfg.configFile
}

func (n *Nmq) GetCertPath() string {
	return n.cfg.certPath
}

func (n *Nmq) GetWorkDir() string {
	return n.cfg.workDir
}

// GetStatus 获取组件状态
func (n *Nmq) GetStatus() nmq.ComponentStatus {
	return n.status
}

// GetContext 获取上下文
func (n *Nmq) GetContext() context.Context {
	return n.ctx
}

func (n *Nmq) GetCancel() context.CancelFunc {
	return n.cancel
}

// GetLogger 获取日志记录器
func (n *Nmq) GetLogger() *zap.Logger {
	return n.logger
}

// SetLogLevel 动态设置日志等级（支持运行时修改）
// 可接受的等级字符串: debug, info, warn, error, dpanic, panic, fatal
func (n *Nmq) SetLogLevel(levelStr string) error {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		return fmt.Errorf("invalid log level %q: %w", levelStr, err)
	}
	n.atomicLevel.SetLevel(level)
	n.logger.Info("log level changed", zap.String("level", level.String()))
	return nil
}

// GetLogLevel 获取当前日志等级
func (n *Nmq) GetLogLevel() string {
	return n.atomicLevel.Level().String()
}

// Execute 运行组件
func (n *Nmq) Execute() error {

	// 注册组件

	n.logger.Info("Waiting for NCP to exit")
	if err := n.rootCmd.Execute(); err != nil {
		n.logger.Error("Failed to execute NCP", zap.Error(err))
		return err
	}

	return nil
}

// RegisterComponent 注册组件
func (n *Nmq) RegisterComponent(componentName string, component nmq.Component) {
	n.mux.Lock()
	defer n.mux.Unlock()
	n.components[componentName] = component
}
