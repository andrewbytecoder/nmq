package ctx

import (
	"go.uber.org/zap"
	"ysp.com/ncp/ncp/interfaces/dpproxy"
	"ysp.com/ncp/ncp/interfaces/ncp"
)

type Context struct {
	log *zap.Logger
	r   dpproxy.IRouter
}

func NewContext(ctx ncp.Context) *Context {
	return &Context{
		log: ctx.GetLogger(),
	}
}

func (c *Context) SetRouter(r dpproxy.IRouter) {
	c.r = r
}

func (c *Context) GetRouter() dpproxy.IRouter {
	return c.r
}

func (c *Context) GetLogger() *zap.Logger {
	return c.log
}
