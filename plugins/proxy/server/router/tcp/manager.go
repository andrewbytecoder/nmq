package tcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	tcpmuxer "github.com/andrewbytecoder/nmq/plugins/proxy/muxer/tcp"
	"github.com/andrewbytecoder/nmq/plugins/proxy/server/provider"
	tcpservice "github.com/andrewbytecoder/nmq/plugins/proxy/server/service/tcp"
	"github.com/andrewbytecoder/nmq/plugins/proxy/tcp"
	nmqtls "github.com/andrewbytecoder/nmq/plugins/proxy/tls"
	"go.uber.org/zap"
)

const maxUserPriority = math.MaxInt - 1000

type middlewareBuilder interface {
	BuildChain(ctx context.Context, names []string) *tcp.Chain
}

// Manager is a route/router manager
type Manager struct {
	log                 *zap.Logger
	serviceManager      *tcpservice.Manager
	middlewareBuilder   middlewareBuilder
	httpHandlers        map[string]http.Handler
	httpsHandler        map[string]http.Handler
	tlsManager          *nmqtls.Manager
	conf                *runtimecfg.Configuration
	providersPrecedence []string
}

func NewManager(log *zap.Logger, conf *runtimecfg.Configuration,
	serviceManager *tcpservice.Manager,
	builder middlewareBuilder,
	httpHandlers map[string]http.Handler,
	httpsHandlers map[string]http.Handler,
	tlsManager *nmqtls.Manager,
	providersPrecedence []string,
) *Manager {
	return &Manager{
		log:                 log,
		serviceManager:      serviceManager,
		middlewareBuilder:   builder,
		httpHandlers:        httpsHandlers,
		httpsHandler:        httpsHandlers,
		tlsManager:          tlsManager,
		conf:                conf,
		providersPrecedence: providersPrecedence,
	}
}

func (m *Manager) addTCPHandlers(ctx context.Context, configs map[string]*runtimecfg.TCPRouterInfo, router *Router) {
	for routerName, routerConfig := range configs {
		m.log.Info("add tcp handler", zap.String("router name", routerName))
		// 将routerName 附加到ctx上
		cxtRouter := provider.AddInContext(ctx, routerName)

		if routerConfig.Priority == 0 {
			routerConfig.Priority = tcpmuxer.GetRulePriority(routerConfig.Rule)
		}

		if routerConfig.Service == "" {
			err := errors.New("the service is missing on the router")
			routerConfig.AddError(err, true)
			m.log.Error("add tcp handler failed", zap.Error(err))
			continue
		}

		if routerConfig.Rule == "" {
			err := errors.New("router has no rule")
			routerConfig.AddError(err, true)
			m.log.Error("add tcp rule failed", zap.Error(err))
			continue
		}

		domains, err := tcpmuxer.ParseHostSNI(routerConfig.Rule)
		if err != nil {
			routerErr := fmt.Errorf("invalid rule: %q , %w", routerConfig.Rule, err)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(err))
			continue
		}

	}
}

func (m *Manager) buildTCPHandler(ctx context.Context, router *runtimecfg.TCPRouterInfo) (tcp.Handler, error) {
	var qualifiedName []string
	for _, name := range router.Middlewares {
		qualifiedName = append(qualifiedName, provider.GetQualifiedName(ctx, name))
	}

	router.Middlewares = qualifiedName

	if router.Service == "" {
		return nil, errors.New("the service is missing on the router")
	}

	sHandler, err := m.serviceManager.BuildTCP(ctx, router.Service)
	if err != nil {
		return nil, fmt.Errorf("build tcp handler failed %w", err)
	}

	mHandler := m.middlewareBuilder.BuildChain(ctx, router.Middlewares)

	return tcp.NewChain().Extend(*mHandler).Then(sHandler)
}

func providerName(routerName string) string {
	parts := strings.Split(routerName, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
