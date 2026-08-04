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
	httpmuxer "github.com/andrewbytecoder/nmq/plugins/proxy/muxer/http"
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
		httpHandlers:        httpHandlers,
		httpsHandler:        httpsHandlers,
		tlsManager:          tlsManager,
		conf:                conf,
		providersPrecedence: providersPrecedence,
	}
}

// BuildHandlers builds the handlers for the given entrypoints.
func (m *Manager) BuildHandlers(rootCtx context.Context, entryPoints []string) map[string]*Router {
	entryPointsRouters := m.getTCPRouters(rootCtx, entryPoints)
	entryPointsRoutersHTTP := m.getHTTPRouters(rootCtx, entryPoints, false)

	entryPointsHandlers := make(map[string]*Router)
	for _, entryPointName := range entryPoints {
		routers := entryPointsRouters[entryPointName]

		handler, err := m.buildEntryPointHandler(rootCtx, routers, entryPointsRoutersHTTP[entryPointName], m.httpHandlers[entryPointName], m.httpsHandler[entryPointName])
		if err != nil {
			m.log.Error("failed to build entry point handler", zap.Error(err))
			continue
		}
		entryPointsHandlers[entryPointName] = handler
	}

	return entryPointsHandlers
}

func (m *Manager) getTCPRouters(ctx context.Context, entryPoints []string) map[string]map[string]*runtimecfg.TCPRouterInfo {
	if m.conf != nil {
		return m.conf.GetTCPRoutersByEntryPoints(ctx, entryPoints)
	}

	return make(map[string]map[string]*runtimecfg.TCPRouterInfo)
}

func (m *Manager) getHTTPRouters(ctx context.Context, entryPoints []string, tls bool) map[string]map[string]*runtimecfg.RouterInfo {
	if m.conf != nil {
		return m.conf.GetRoutersByEntryPoints(ctx, entryPoints, tls)
	}

	return make(map[string]map[string]*runtimecfg.RouterInfo)
}

func (m *Manager) buildEntryPointHandler(ctx context.Context, configs map[string]*runtimecfg.TCPRouterInfo,
	configsHTTP map[string]*runtimecfg.RouterInfo, handlerHTTP, handlerHTTPS http.Handler) (*Router, error) {

	// Build a new Router
	router, err := NewRouter(m.log, m.providersPrecedence)
	if err != nil {
		return nil, err
	}

	router.SetHTTPHandler(handlerHTTP)

	// Even though the error is seemingly ignored (aside from logging it),
	// we actually rely later on the fact that a tls config is nil (which happens when an error is returned) to take special steps
	// when assigning a handler to a route.
	defaultTLSConf, err := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, nmqtls.DefaultTLSConfigName)
	if err != nil {
		m.log.Error("failed to get default tls config", zap.Error(err))
	}

	for routerHTTPName, routerHTTPConfig := range configsHTTP {
		if routerHTTPConfig.TLS == nil {
			continue
		}

		m.log.Info("add http handler", zap.String("router name", routerHTTPName))
		ctxRouter := provider.AddInContext(ctx, routerHTTPName)

		// Even if the TLS options mismatch between the configured and the resolved one is handled in the aggregator
		// we also have to handle it here to be able to mark the router in error
		tlsOptionsName := nmqtls.DefaultTLSConfigName
		if len(routerHTTPConfig.TLS.Options) > 0 && routerHTTPConfig.TLS.Options != nmqtls.DefaultTLSConfigName {
			tlsOptionsName = provider.GetQualifiedName(ctxRouter, routerHTTPConfig.TLS.Options)
		}

		domains, err := httpmuxer.ParseDomains(routerHTTPConfig.Rule)
		if err != nil {
			routerErr := fmt.Errorf("invalid rule %s, error: %w", routerHTTPConfig.Rule, err)
			routerHTTPConfig.AddError(routerErr, true)
			m.log.Error("add http rule failed", zap.Error(routerErr))
			continue
		}

		if len(domains) == 0 {
			// Extra Host(*) rule, for HTTPS routers with no Host rule
			// and for requests for which the SNI does not match _any_ of the other existing routers Host
			// This is only about choosing the TLS configuretion
			// The actual routing will be done further on by the HTTPS handler
			// See examples below
			router.AddHTTPTLSConfig("*", defaultTLSConf, nmqtls.DefaultTLSConfigName)

			// The server name (from a Host(SNI) rule) is the only parameter (available in HTTP routing rules) on which we can map a TLS config,
			// because it is the only one accessible before decryption (we obtain it during the ClientHello).
			// Therefore, when a router has no Host rule, it does not make any sense to specify some TLS options.
			// Consequently, when it comes to deciding what TLS config will be used,
			// for a request that will match an HTTPS router with no Host rule,
			// the result will depend on the _others_ existing routers (their Host rule, to be precise), and the TLS options associated with them,
			// even though they don't match the incoming request. Consider the following examples:

			//	# conf1
			//	httpRouter1:
			//		rule: PathPrefix("/foo")
			//	# Wherever the request comes from, the TLS config used will be the default one, because of the Host(*) fallback.

			//	# conf2
			//	httpRouter1:
			//		rule: PathPrefix("/foo")
			//
			//	httpRouter2:
			//		rule: Host("foo.com") && PathPrefix("/bar")
			//		tlsoptions: myTLSOptions
			//	# When a request for "/foo" comes, even though it won't be routed by httpRouter2,
			//	# if its SNI is set to foo.com, myTLSOptions will be used for the TLS connection.
			//	# Otherwise, it will fallback to the default TLS config.
			if tlsOptionsName != nmqtls.DefaultTLSConfigName {
				m.log.Error("no domain found in rule, the TLS option cannot be applied", zap.String(routerHTTPConfig.Rule, tlsOptionsName))
				routerHTTPConfig.AddError(fmt.Errorf("no domain found in rule %v, the TLS option %s cannot be applied", routerHTTPConfig.Rule, tlsOptionsName), false)
			}
		}

		if len(domains) > 0 && routerHTTPConfig.TLS.ResolvedOptions != tlsOptionsName {
			routerHTTPConfig.AddError(errors.New("router's TLSOptions configuration is conflicting with other routers on the same entrypoint and host, default TLS options will be used instead"), false)
		}

		// Even though the error is seemingly ignored (aside from logging it),
		// we actually rely later on the fact that a tls config is nil (which happens when an error is returned) to take special steps
		// when assigning a handler to a route.
		tlsConf, tlsConfErr := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, routerHTTPConfig.TLS.ResolvedOptions)
		if tlsConfErr != nil {
			// Note: we do not call AddError here because we already did so when buildRouterHandler errored for the same reason.
			m.log.Error("failed to get tls config", zap.Error(tlsConfErr))
		}
		for _, domain := range domains {
			if tlsConf == nil {
				// we use nil config as a signal to insert a handler
				// that enforces that TLS connection attempts to the corresponding (broken) router should fail.
				m.log.Error("failed to get tls config", zap.Error(tlsConfErr))
				router.AddHTTPTLSConfig(domain, nil, "")
				continue
			}

			m.log.Info("add http handler", zap.String("domain", domain), zap.String("tls options", routerHTTPConfig.TLS.ResolvedOptions))
			router.AddHTTPTLSConfig(domain, tlsConf, routerHTTPConfig.TLS.ResolvedOptions)
		}

	}

	// Keep in mind that defaultTLSConf might be nil here.
	router.SetHTTPSHandler(handlerHTTPS, defaultTLSConf)

	m.addTCPHandlers(ctx, configs, router)

	return router, nil
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
