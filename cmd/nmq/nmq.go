package main

import (
	"fmt"
	"os"

	"github.com/andrewbytecoder/nmq/pkg/utils"
	"github.com/andrewbytecoder/nmq/plugins/nmq"
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
	log, atomicLevel, err := utils.CreateProductZapLogger(utils.SetLogLevel(zapcore.DebugLevel),
		utils.SetLogMaxSize(50), utils.SetLogMaxBackups(2),
		utils.SetLogMaxAge(30), utils.SetLogCompress(true),
		utils.SetLogFilename("./log/nmq.log"), utils.SetLogLevelKey("nmq"),
		utils.SetConsoleWriterSyncer(true))
	if err != nil {
		fmt.Println("Failed to create logger")
		return
	}

	run := nmq.NewNmq(
		nmq.SetLogger(log),              // 赋能日志记录器
		nmq.SetAtomicLevel(atomicLevel), // 设置日志原子级别，支持运行时动态修改
		nmq.SetDebugPort(6060),          // 调试端口，用于动态日志等级等
		nmq.SetEnableGoPs(true),         // 赋能gops
		nmq.SetEnablePyroscope(true),    // 赋能pyroscope
	)
	err = run.Execute()
	if err != nil {
		fmt.Println("Failed to execute nmq")
		return
	}
}
