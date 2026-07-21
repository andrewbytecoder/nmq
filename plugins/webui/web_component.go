package webui

import (
	"fmt"

	"go.uber.org/zap"
	"ysp.com/ncp/ncp/interfaces"
	"ysp.com/ncp/ncp/interfaces/ncp"
	"ysp.com/ncp/ncp/internal/config"
	"ysp.com/ncp/ncp/plugins/webui/api/dashboard"
	"ysp.com/ncp/ncp/plugins/webui/storage"
)

const componentName = "webui"

type Component struct {
	ncp.ComponentBase
}

func NewWebComponent(ctx ncp.Context) *Component {
	component := &Component{
		ComponentBase: ncp.NewComponentBase(ctx),
	}

	return component
}

func NewNetComponent(ctx ncp.Context) *Component {
	return NewWebComponent(ctx)
}

func (c *Component) GetInterface(uuid string) any {
	return nil
}

func (c *Component) Init() error {
	return nil
}

func (c *Component) Start() error {
	go func() {
		err := c.run()
		if err != nil {
			c.Log.Error("start webui plugin failed", zap.Error(err))
			return
		}
	}()

	return nil
}

func (c *Component) Stop() error {
	return nil
}

func (c *Component) Reset() error {
	return nil
}

func (c *Component) GetName() string {
	return interfaces.WebComponentName
}

func (c *Component) GetVersion() string {
	return "1.0.0"
}

func (c *Component) Notify(event string, data any) {
}

func (c *Component) GetStatus() ncp.ComponentStatus {
	return c.Status
}

func (c *Component) run() error {
	listenAddr := config.GetWebHttpAddress()

	server, err := dashboard.NewServer(dashboard.Config{
		Logger:          c.Log,
		DashboardPath:   "/dashboard",
		APIBasePath:     "/api",
		ListenAddr:      listenAddr,
		StorageProvider: storage.NewSQLiteTableProvider(c.NcpCtx, c.Log),
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
