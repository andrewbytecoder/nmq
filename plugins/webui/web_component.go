package webui

import (
	"fmt"

	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/interfaces/nmq"
	"github.com/andrewbytecoder/nmq/internal/config"
	"github.com/andrewbytecoder/nmq/plugins/webui/api"
	"github.com/andrewbytecoder/nmq/plugins/webui/ctx"
	"github.com/andrewbytecoder/nmq/plugins/webui/routeinfoprovider"
	"github.com/andrewbytecoder/nmq/plugins/webui/storage"
	"go.uber.org/zap"
)

const componentName = "webui"

type WebUI struct {
	nmq.ComponentBase

	ctx *ctx.Context
}

func NewWebComponent(ctx nmq.Context) *WebUI {
	component := &WebUI{
		ComponentBase: nmq.NewComponentBase(ctx),
	}

	return component
}

func (c *WebUI) GetInterface(uuid string) any {
	return nil
}

func (c *WebUI) Init() error {
	// 1. 初始化上下文环境
	c.ctx = ctx.NewContext(c.NmqCtx)

	return nil
}

func (c *WebUI) Start() error {
	// 2. 初始化路由管理器
	r, ok := c.NmqCtx.GetInterface(interfaces.NetworkComponentName).(gin.IRouter)
	if !ok {
		c.Log.Error("get router manager failed")
		return fmt.Errorf("get router manager failed")
	}
	c.ctx.SetRouter(r)

	go func() {
		err := c.run()
		if err != nil {
			c.Log.Error("start webui plugin failed", zap.Error(err))
			return
		}
	}()

	return nil
}

func (c *WebUI) Stop() error {
	return nil
}

func (c *WebUI) Reset() error {
	return nil
}

func (c *WebUI) GetName() string {
	return interfaces.WebComponentName
}

func (c *WebUI) GetVersion() string {
	return "1.0.0"
}

func (c *WebUI) Notify(event string, data any) {
}

func (c *WebUI) GetStatus() nmq.ComponentStatus {
	return c.Status
}

func (c *WebUI) run() error {
	listenAddr := config.GetNetworkConfig().GetWebuiConfig().GetAddress()

	server, err := api.NewServer(api.Config{
		Ctx:             c.ctx,
		Logger:          c.Log,
		DashboardPath:   "/dashboard",
		APIBasePath:     "/api",
		ListenAddr:      listenAddr,
		StorageProvider: storage.NewSQLiteTableProvider(c.NmqCtx, c.Log),
		RouteProvider:   routeinfoprovider.New(c.ctx.GetRouter()),
	})
	if err != nil {
		c.Log.Error("create webui server failed", zap.Error(err))
		return err
	}

	c.Log.Info("starting webui plugin",
		zap.String("listenAddr", listenAddr),
		zap.String("dashboardURL", fmt.Sprintf("http://%s/dashboard/", listenAddr)),
	)

	return server.Run(listenAddr)
}
