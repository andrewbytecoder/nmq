package ctx

import (
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"go.uber.org/zap"
)

type Context struct {
	log *zap.Logger
	r   gin.IRouter
}

func NewContext(ctx nmq.Context) *Context {
	return &Context{
		log: ctx.GetLogger(),
	}
}

func (c *Context) SetRouter(r gin.IRouter) {
	c.r = r
}

func (c *Context) GetRouter() gin.IRouter {
	return c.r
}

func (c *Context) GetLogger() *zap.Logger {
	return c.log
}
