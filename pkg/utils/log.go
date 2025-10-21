package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogConfig struct {
	lumberjackLogger *lumberjack.Logger // 文件日志写入器，支持自动切割
	level            zapcore.Level      // 日志输出级别（debug/info/warn/error/fatal
	levelKey         string             // JSON 输出中表示日志级别的字段名。
	consoleWriter    bool               // 是否将日志输出到控制台
}

// An Option configures a Logger.
type Option interface {
	apply(*LogConfig)
}

// optionFunc wraps a func so it satisfies the Option interface.
type optionFunc func(*LogConfig)

func (f optionFunc) apply(k *LogConfig) {
	f(k)
}

// SetLogFilename 设置日志文件名
func SetLogFilename(filename string) Option {
	return optionFunc(func(c *LogConfig) {
		c.lumberjackLogger.Filename = filename
	})
}

// SetLogMaxSize 设置日志文件最大大小（MB）
func SetLogMaxSize(maxSize int) Option {
	return optionFunc(func(c *LogConfig) {
		c.lumberjackLogger.MaxSize = maxSize
	})
}

// SetLogMaxAge 设置日志文件最大保存天数
func SetLogMaxAge(maxAge int) Option {
	return optionFunc(func(c *LogConfig) {
		c.lumberjackLogger.MaxAge = maxAge
	})
}

// SetLogMaxBackups 设置日志文件最大备份数
func SetLogMaxBackups(maxBackups int) Option {
	return optionFunc(func(c *LogConfig) {
		c.lumberjackLogger.MaxBackups = maxBackups
	})
}

// SetLogCompress 设置是否压缩日志文件
func SetLogCompress(compress bool) Option {
	return optionFunc(func(c *LogConfig) {
		c.lumberjackLogger.Compress = compress
	})
}

// SetLogLevel 设置日志级别
func SetLogLevel(level zapcore.Level) Option {
	return optionFunc(func(c *LogConfig) {
		c.level = level
	})
}

// SetLogLevelKey 设置日志级别字段名
func SetLogLevelKey(levelKey string) Option {
	return optionFunc(func(c *LogConfig) {
		c.levelKey = levelKey
	})
}

func SetConsoleWriterSyncer(consoleWriter bool) Option {
	return optionFunc(func(c *LogConfig) {
		c.consoleWriter = consoleWriter
	})
}

// CreateProductZapLogger 创建一个生产级别的 zap 日志记录器。
func CreateProductZapLogger(op ...Option) (*zap.Logger, error) {
	logConfig := &LogConfig{
		lumberjackLogger: &lumberjack.Logger{},
	}

	for _, opt := range op {
		opt.apply(logConfig)
	}

	// 创建 zap 的核心配置
	fileWriteSyncer := zapcore.AddSync(logConfig.lumberjackLogger)

	var multiWriteSyncer zapcore.WriteSyncer
	// 组合写入器
	if logConfig.consoleWriter {
		consoleWriteSyncer := zapcore.AddSync(os.Stdout)
		multiWriteSyncer = zapcore.NewMultiWriteSyncer(fileWriteSyncer, consoleWriteSyncer)
	} else {
		multiWriteSyncer = zapcore.NewMultiWriteSyncer(fileWriteSyncer)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 时间格式
	encoderConfig.LevelKey = logConfig.levelKey
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig), // 使用 JSON 格式编码日志
		multiWriteSyncer,
		logConfig.level, // 设置日志级别
	)

	// 创建 zap logger
	logger := zap.New(core, zap.AddCaller()) // 添加调用者信息
	return logger, nil
}
