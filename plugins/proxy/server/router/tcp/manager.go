package tcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	"github.com/andrewbytecoder/nmq/plugins/proxy/muxer"
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

		// HostSNI Rule, but TLS not set on the router, which is an error
		// However, we allow the HostSNI(*) exception
		if len(domains) > 0 && routerConfig.TLS == nil && domains[0] != "*" {
			routerErr := fmt.Errorf("invalid rule: %q , has HostSNI matcher, but no TLS on router", routerConfig.Rule)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(routerErr))
			continue
		}

		if routerConfig.Priority > maxUserPriority && !strings.HasSuffix(routerName, "@internal") {
			routerErr := fmt.Errorf("the router priority %d exceeds the max user-defined priority %d", routerConfig.Priority, maxUserPriority)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(routerErr))
			continue
		}

		var handler tcp.Handler
		if routerConfig.TLS == nil || routerConfig.TLS.Passthrough {
			handler, err = m.buildTCPHandler(cxtRouter, routerConfig)
			if err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp handler failed", zap.Error(err))
				continue
			}
		}

		if routerConfig.TLS == nil {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))

			if err = router.muxerTCP.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp rule failed", zap.Error(err))
			}
			continue
		}

		if routerConfig.TLS.Passthrough {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))

			if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp tls rule failed")
			}
			continue
		}

		for _, domain := range domains {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))
			if muxer.IsASCII(domain) {
				continue
			}

			// 保证domain中配置的都是ascii
			asciiError := fmt.Errorf("invalid domain name value %q, non-ASCUU characters are not allowed", domain)
			routerConfig.AddError(asciiError, true)
			m.log.Error("add tcp tls rule failed", zap.Error(asciiError))
		}

		tlsOptionsName := routerConfig.TLS.Options

		if len(tlsOptionsName) == 0 {
			tlsOptionsName = nmqtls.DefaultTLSConfigName
		}

		if tlsOptionsName != nmqtls.DefaultTLSConfigName {
			tlsOptionsName = provider.GetQualifiedName(cxtRouter, tlsOptionsName)
		}

		tlsConf, err := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, tlsOptionsName)
		if err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.String(routerConfig.Rule, tlsOptionsName), zap.Error(err))

			if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), &brokenTLSRouter{}); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp tls rule failed", zap.Error(err))
			}

			continue
		}

		// Now that the Rule is not just about the Host, we could theoretically have a config like:
		//	router1:
		//		rule: HostSNI(foo.com) && ClientIP(IP1)
		//		tlsOption: tlsOne
		//	router2:
		//		rule: HostSNI(foo.com) && ClientIP(IP2)
		//		tlsOption: tlsTwo
		// i.e. same HostSNI but different tlsOptions
		// This is only applicable if the muxer can decide about the routing _before_ telling the client about the tlsConf (i.e. before the TLS HandShake).
		// This seems to be the case so far with the existing matchers (HostSNI, and ClientIP), so it's all good.
		// Otherwise, we would have to do as for HTTPS, i.e. disallow different TLS configs for the same HostSNIs.

		handler, err = m.buildTCPHandler(cxtRouter, routerConfig)
		if err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.Error(err))
			continue
		}

		handler = &tcp.TLSHandler{
			Next:           handler,
			Config:         tlsConf,
			TLSOptionsName: tlsOptionsName,
		}

		m.log.Debug("Add route for", zap.String("rule", routerConfig.Rule), zap.String("tlsOptionsName", tlsOptionsName))

		if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.Error(err))
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
