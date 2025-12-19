package utils

import (
	"bytes"
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapWriter 将 io.Writer 接口桥接到 zap.Logger
type ZapWriter struct {
	logger *zap.Logger
	level  zapcore.Level
	buffer []byte
}

// NewZapWriter 创建一个新的 io.Writer，输出到 zap.Logger
// level: 日志级别 (zap.InfoLevel, zap.WarnLevel 等)
func NewZapWriter(logger *zap.Logger, level zapcore.Level) io.Writer {
	return &ZapWriter{
		logger: logger,
		level:  level,
		buffer: make([]byte, 0, 1024*10), // 预分配缓冲区
	}
}

// Write 实现 io.Writer 接口
func (w *ZapWriter) Write(p []byte) (n int, err error) {
	// 追加数据到缓冲区
	w.buffer = append(w.buffer, p...)

	// 如果有换行符，按行分割并输出
	for {
		i := bytes.IndexByte(w.buffer, '\n')
		if i < 0 {
			break // 没有完整的一行，等待更多数据
		}

		// 提取一行（不包含 \n）
		line := w.buffer[:i]
		w.buffer = w.buffer[i+1:] // 移除已处理的部分

		// 使用 zap 输出
		switch w.level {
		case zapcore.DebugLevel:
			w.logger.Debug(string(line))
		case zapcore.InfoLevel:
			w.logger.Info(string(line))
		case zapcore.WarnLevel:
			w.logger.Warn(string(line))
		case zapcore.ErrorLevel:
			w.logger.Error(string(line))
		case zapcore.DPanicLevel:
			w.logger.DPanic(string(line))
		case zapcore.PanicLevel:
			w.logger.Panic(string(line))
		case zapcore.FatalLevel:
			w.logger.Fatal(string(line))
		default:
			w.logger.Info(string(line))
		}
	}

	return len(p), nil
}

// Flush 将剩余缓冲区内容输出（例如程序退出时调用）
func (w *ZapWriter) Flush() {
	if len(w.buffer) > 0 {
		line := string(w.buffer)
		switch w.level {
		case zapcore.DebugLevel:
			w.logger.Debug(line)
		case zapcore.InfoLevel:
			w.logger.Info(line)
		case zapcore.WarnLevel:
			w.logger.Warn(line)
		default:
			w.logger.Info(line)
		}
		w.buffer = w.buffer[:0] // 清空
	}
}
