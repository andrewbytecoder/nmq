package nmq

import (
	"context"

	"go.uber.org/zap"
)

// An Option configures a Logger.
type Option interface {
	apply(nmq *Nmq)
}

// optionFunc wraps a func so it satisfies the Option interface.
type optionFunc func(*Nmq)

// apply applies the Option to the given Ncp.
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

func SetEnableGoPs(enableGoPs bool) Option {
	return optionFunc(func(n *Nmq) {
		n.cfg.setGoPs(enableGoPs)
	})
}

func SetEnablePyroscope(enablePyroscope bool) Option {
	return optionFunc(func(n *Nmq) {
		n.cfg.setPyroscope(enablePyroscope)
	})
}
