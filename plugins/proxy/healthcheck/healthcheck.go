package healthcheck

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/andrewbytecoder/nmq/internal/config/dynamic"
	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
)

// StatusSetter 状态设置接口, 用于设置子服务的状态, 服务里面应该实现这个接口，通知对应服务的状态
type StatusSetter interface {
	SetStatus(ctx context.Context, childName string, status bool)
}

// StatusUpdater should be implemented by a service that, when its status
// changes (e.g. all if its children are down), needs to propagate upwards (to
// their parent(s)) that change.
type StatusUpdater interface {
	RegisterStatusUpdater(fn func(up bool)) error
}

type target struct {
	name      string
	targetUrl *url.URL
}

type ServiceHealthChecker struct {
	balancer StatusSetter
	info     *runtimecfg.ServiceInfo

	config            *dynamic.ServerHealthCheck
	interval          time.Duration
	unhealthyInterval time.Duration
	timeout           time.Duration

	client *http.Client

	healthyTargets   chan target
	unhealthyTargets chan target

	serviceName string
}
