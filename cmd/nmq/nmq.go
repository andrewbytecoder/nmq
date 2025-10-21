package main

import (
	"fmt"
	"os"

	"github.com/nmq/interfaces"
	"github.com/nmq/pkg/utils"
	"github.com/nmq/plugins/network"
	"github.com/nmq/plugins/nmq"
	"go.uber.org/zap/zapcore"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Failed to get current directory")
		return
	}
	//nmq.DefaultConfig().SetLogFile(dir + "/nmq.log")
	fmt.Printf("Current working directory: %s\n", dir)

	// 创建日志记录器， 每个 50M, 两个备份，最多三个，备份日志最长保存30天，压缩备份日志
	log, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
		utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
		utils.SetLogMaxAge(30), utils.SetLogCompress(true),
		utils.SetLogFilename("./log/dp-proxy.log"), utils.SetLogLevelKey("dp-proxy"),
		utils.SetConsoleWriterSyncer(true))
	if err != nil {
		fmt.Println("Failed to create logger")
		return
	}

	run := nmq.NewNmq(
		nmq.SetLogger(log),           // 赋能日志记录器
		nmq.SetEnableGoPs(true),      // 赋能gops
		nmq.SetEnablePyroscope(true), // 赋能pyroscope
	)
	RegisterComponents(run)
	err = run.Execute()
	if err != nil {
		fmt.Println("Failed to execute nmq")
		return
	}
}

func RegisterComponents(nmq *nmq.Nmq) {
	// 注册网络插件
	nmq.RegisterComponent(interfaces.NetworkComponentName, network.NewNetComponent(nmq))
}
