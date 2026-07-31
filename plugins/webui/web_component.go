package webui

import (
	"fmt"

	"go.uber.org/zap"
	"ysp.com/ncp/ncp/interfaces"
	"ysp.com/ncp/ncp/interfaces/dpproxy"
	"ysp.com/ncp/ncp/interfaces/ncp"
	"ysp.com/ncp/ncp/internal/config"
	"ysp.com/ncp/ncp/plugins/webui/api"
	"ysp.com/ncp/ncp/plugins/webui/ctx"
	"ysp.com/ncp/ncp/plugins/webui/routeinfoprovider"
	"ysp.com/ncp/ncp/plugins/webui/storage"
)

const componentName = "webui"

type WebUI struct {
	ncp.ComponentBase

	ctx *ctx.Context
}

func NewWebComponent(ctx ncp.Context) *WebUI {
	component := &WebUI{
		ComponentBase: ncp.NewComponentBase(ctx),
	}

	return component
}

func (c *WebUI) GetInterface(uuid string) any {
	return nil
}

func (c *WebUI) Init() error {
	// 1. 初始化上下文环境
	c.ctx = ctx.NewContext(c.NcpCtx)

	return nil
}

func (c *WebUI) Start() error {
	// 2. 初始化路由管理器
	r, ok := c.NcpCtx.GetInterface(interfaces.DpProxyRouter).(dpproxy.IRouter)
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

func (c *WebUI) GetStatus() ncp.ComponentStatus {
	return c.Status
}

func (c *WebUI) run() error {
	listenAddr := config.GetWebHttpAddress()

	server, err := api.NewServer(api.Config{
		Ctx:             c.ctx,
		Logger:          c.Log,
		DashboardPath:   "/dashboard",
		APIBasePath:     "/api",
		ListenAddr:      listenAddr,
		StorageProvider: storage.NewSQLiteTableProvider(c.NcpCtx, c.Log),
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
