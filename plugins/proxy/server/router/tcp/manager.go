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

// maxUserPriority 用户可定义的最大优先级。
// math.MaxInt - 1000 保留给内部路由器（名称以 @internal 结尾的路由器），
// 确保内部路由器的优先级始终高于用户自定义的路由器。
//
// 使用示例:
//
//	用户路由器优先级范围: [0, math.MaxInt - 1000]
//	内部路由器优先级范围: (math.MaxInt - 1000, math.MaxInt]
const maxUserPriority = math.MaxInt - 1000

// middlewareBuilder 定义中间件链构建器接口。
// 根据中间件名称列表，构建一条 TCP 中间件调用链。
//
// 实现示例:
//
//	type myBuilder struct{}
//	func (b *myBuilder) BuildChain(ctx context.Context, names []string) *tcp.Chain {
//	    chain := tcp.NewChain()
//	    for _, name := range names {
//	        chain = chain.Append(middlewareRegistry.Get(name))
//	    }
//	    return &chain
//	}
type middlewareBuilder interface {
	BuildChain(ctx context.Context, names []string) *tcp.Chain
}

// Manager 是路由/路由器管理器。
//
// 负责将运行时配置中的路由定义转换为可用的 Router 实例。
// 每个 EntryPoint 对应一个 Router，Router 内部包含三个 muxer：
//   - muxerTCP:    纯 TCP 路由匹配
//   - muxerTCPTLS: 带 TLS 终止的 TCP 路由匹配
//   - muxerHTTPS:  HTTPS (HTTP over TLS) 路由匹配
//
// 字段说明:
//   - log:                 日志记录器
//   - serviceManager:      TCP 服务管理器，用于构建后端服务处理器
//   - middlewareBuilder:   中间件链构建器
//   - httpHandlers:        HTTP 入口处理器映射 (entryPointName → Handler)
//   - httpsHandler:        HTTPS 入口处理器映射 (entryPointName → Handler)
//   - tlsManager:          TLS 证书管理器
//   - conf:                运行时配置，包含所有路由定义
//   - providersPrecedence: Provider 优先级顺序列表，决定路由冲突时的优先级
//
// 使用示例:
//
//	mgr := NewManager(
//	    log,
//	    runtimeConf,
//	    serviceManager,
//	    middlewareBuilder,
//	    map[string]http.Handler{"web": httpHandler},
//	    map[string]http.Handler{"web": httpsHandler},
//	    tlsManager,
//	    []string{"file", "docker", "k8s"},
//	)
//	routers := mgr.BuildHandlers(ctx, []string{"web", "websecure"})
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

// NewManager 创建一个新的路由管理器。
//
// 参数说明:
//   - log:                  zap 日志记录器
//   - conf:                 运行时配置（可为 nil，为 nil 时不加载任何路由）
//   - serviceManager:       TCP 服务管理器
//   - builder:              中间件链构建器
//   - httpHandlers:         HTTP 入口处理器映射，key 为 entryPoint 名称
//   - httpsHandlers:        HTTPS 入口处理器映射，key 为 entryPoint 名称
//   - tlsManager:           TLS 证书管理器
//   - providersPrecedence:  Provider 优先级列表，如 []string{"k8s", "file", "default"}
//
// 使用示例:
//
//	mgr := NewManager(
//	    log,
//	    runtimeConf,
//	    serviceManager,
//	    builder,
//	    map[string]http.Handler{"web": httpHandler},
//	    map[string]http.Handler{"web": httpsHandler},
//	    tlsManager,
//	    []string{"k8s", "file"},
//	)
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

// BuildHandlers 为指定的 EntryPoints 构建路由器映射。
//
// 流程:
//  1. 从运行时配置中获取 TCP 和 HTTP 路由定义
//  2. 遍历每个 EntryPoint，构建对应的 Router
//  3. Router 内部包含三个 muxer（TCP/TCP-TLS/HTTPS），统一处理该入口的所有流量
//
// 参数:
//   - rootCtx:     根上下文，传递到路由构建的整个生命周期
//   - entryPoints: 需要构建路由器的入口点名称列表，如 []string{"web", "websecure"}
//
// 返回值: map[entryPointName]*Router，每个 EntryPoint 对应一个 Router 实例
//
// 使用示例:
//
//	routers := mgr.BuildHandlers(ctx, []string{"web", "websecure"})
//	webRouter := routers["web"]
//	webRouter.ServeTCP(conn) // 使用路由器处理 TCP 连接
func (m *Manager) BuildHandlers(rootCtx context.Context, entryPoints []string) map[string]*Router {
	// 获取所有 EntryPoint 的 TCP 路由配置
	entryPointsRouters := m.getTCPRouters(rootCtx, entryPoints)
	// 获取所有 EntryPoint 的 HTTP 路由配置 (tls=false 获取非 HTTPS 的 HTTP 路由)
	entryPointsRoutersHTTP := m.getHTTPRouters(rootCtx, entryPoints, false)

	entryPointsHandlers := make(map[string]*Router)
	for _, entryPointName := range entryPoints {
		routers := entryPointsRouters[entryPointName]

		handler, err := m.buildEntryPointHandler(rootCtx, routers, entryPointsRoutersHTTP[entryPointName], m.httpHandlers[entryPointName], m.httpsHandler[entryPointName])
		// 如果构建失败:
		// - 条件成立: buildEntryPointHandler 返回了错误（例如 Router 创建失败）
		// - 处理: 记录错误日志，跳过该 EntryPoint，不将其加入返回的 map
		// - 含义: 该 EntryPoint 将没有可用的路由器，所有到达该入口的连接都会失败
		if err != nil {
			m.log.Error("failed to build entry point handler", zap.Error(err))
			continue
		}
		entryPointsHandlers[entryPointName] = handler
	}

	return entryPointsHandlers
}

// getTCPRouters 从运行时配置中获取指定 EntryPoints 的所有 TCP 路由。
//
// 返回值: map[entryPointName]map[routerName]*TCPRouterInfo
//
// 条件判断:
//   - m.conf != nil: 配置已加载，从配置中读取路由定义
//   - m.conf == nil: 配置未加载（如测试环境或初始化阶段），返回空 map
//
// 使用示例:
//
//	tcpRouters := m.getTCPRouters(ctx, []string{"tcp-entry"})
//	// tcpRouters = {"tcp-entry": {"my-tcp-router@file": &TCPRouterInfo{...}}}
func (m *Manager) getTCPRouters(ctx context.Context, entryPoints []string) map[string]map[string]*runtimecfg.TCPRouterInfo {
	// 如果配置非空: 从运行时配置中按 EntryPoint 过滤并返回 TCP 路由
	// 如果配置为空: 返回空 map，避免空指针访问
	if m.conf != nil {
		return m.conf.GetTCPRoutersByEntryPoints(ctx, entryPoints)
	}

	return make(map[string]map[string]*runtimecfg.TCPRouterInfo)
}

// getHTTPRouters 从运行时配置中获取指定 EntryPoints 的所有 HTTP 路由。
//
// 参数:
//   - tls: 是否只获取启用了 TLS 的 HTTP 路由 (true=仅HTTPS路由, false=所有HTTP路由)
//
// 返回值: map[entryPointName]map[routerName]*RouterInfo
//
// 条件判断:
//   - m.conf != nil: 配置已加载，从配置中读取路由定义
//   - m.conf == nil: 配置未加载，返回空 map
//
// 使用示例:
//
//	// 获取所有 HTTP 路由（不限定 TLS）
//	allHTTP := m.getHTTPRouters(ctx, []string{"web"}, false)
//
//	// 仅获取启用 TLS 的 HTTPS 路由
//	httpsOnly := m.getHTTPRouters(ctx, []string{"web"}, true)
func (m *Manager) getHTTPRouters(ctx context.Context, entryPoints []string, tls bool) map[string]map[string]*runtimecfg.RouterInfo {
	// 如果配置非空: 从运行时配置中按 EntryPoint 过滤并返回 HTTP 路由
	// 如果配置为空: 返回空 map
	if m.conf != nil {
		return m.conf.GetRoutersByEntryPoints(ctx, entryPoints, tls)
	}

	return make(map[string]map[string]*runtimecfg.RouterInfo)
}

// buildEntryPointHandler 为单个 EntryPoint 构建 Router。
//
// 职责:
//  1. 创建新的 Router 实例（包含 TCP/TCP-TLS/HTTPS 三个 muxer）
//  2. 注册 HTTP 路由的 TLS 配置（通过域名 → TLS 配置映射，用于 ClientHello 阶段选择正确的证书）
//  3. 注册 TCP 路由处理器
//
// 参数:
//   - ctx:          上下文
//   - configs:      该 EntryPoint 的 TCP 路由配置映射 (routerName → TCPRouterInfo)
//   - configsHTTP:  该 EntryPoint 的 HTTP 路由配置映射 (routerName → RouterInfo)
//   - handlerHTTP:  HTTP 请求的最终处理器（非 TLS 的 HTTP 流量）
//   - handlerHTTPS: HTTPS 请求的最终处理器（经过 TLS 解密后的 HTTP 流量）
//
// 使用示例:
//
//	router, err := m.buildEntryPointHandler(
//	    ctx,
//	    tcpRoutersMap,
//	    httpRoutersMap,
//	    httpHandler,    // 处理 http:// 流量
//	    httpsHandler,   // 处理 https:// 流量
//	)
func (m *Manager) buildEntryPointHandler(ctx context.Context, configs map[string]*runtimecfg.TCPRouterInfo,
	configsHTTP map[string]*runtimecfg.RouterInfo, handlerHTTP, handlerHTTPS http.Handler) (*Router, error) {

	// 创建新的 Router 实例（内部初始化三个 muxer）
	router, err := NewRouter(m.log, m.providersPrecedence)
	// 如果 Router 创建失败:
	// - 条件成立: 无法创建 Router（极端情况，通常是内存不足）
	// - 处理: 向上返回错误，调用者会跳过该 EntryPoint
	if err != nil {
		return nil, err
	}

	// 注册 HTTP 处理器：非 TLS 的 HTTP 连接（如 http:// 直接访问）将由 handlerHTTP 处理
	router.SetHTTPHandler(handlerHTTP)

	// 获取默认 TLS 配置。
	// 注意：即使获取失败（err != nil），我们也忽略错误继续执行。
	// 这并非 bug —— 后续代码依赖 defaultTLSConf 为 nil 作为信号，
	// 表示默认 TLS 配置不可用，Router 会采取特殊处理（插入 brokenTLSRouter）。
	defaultTLSConf, err := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, nmqtls.DefaultTLSConfigName)
	// 如果获取默认 TLS 配置失败:
	// - 条件成立: TLS 管理器中没有默认证书配置
	// - 处理: 仅记录错误日志，defaultTLSConf 保持为 nil
	// - 影响: 后续没有 Host 规则的 HTTPS 路由将使用 nil TLS 配置（连接会失败）
	if err != nil {
		m.log.Error("failed to get default tls config", zap.Error(err))
	}

	// ---- 第一阶段：注册 HTTP 路由的 TLS 配置 ----
	// 遍历所有 HTTP 路由，根据 Host(SNI) 规则为每个域名注册对应的 TLS 证书配置。
	// 这样在 TLS ClientHello 阶段，Router 就能根据 SNI 选择正确的证书。
	for routerHTTPName, routerHTTPConfig := range configsHTTP {
		// 条件: routerHTTPConfig.TLS == nil
		// 成立: 该 HTTP 路由没有配置 TLS，不需要建立 TLS 连接
		//       （此路由只能用于纯 HTTP EntryPoint，不应出现在 HTTPS EntryPoint）
		// 处理: 直接跳过，不注册 TLS 配置
		// 示例: 一个只配置了 rule: PathPrefix("/api") 的路由，没有 tls: {} 配置块
		if routerHTTPConfig.TLS == nil {
			continue
		}

		m.log.Info("add http handler", zap.String("router name", routerHTTPName))
		// 将路由名称附加到上下文中，用于生成完整的限定名
		ctxRouter := provider.AddInContext(ctx, routerHTTPName)

		// 确定要使用的 TLS 配置选项名称。
		// 流程:
		//   1. 默认使用 DefaultTLSConfigName ("default")
		//   2. 如果用户配置了非空的 TLS Options 且不是 "default"，则将其转为完整限定名
		//
		// 为什么需要 TLS Options 的冲突检测:
		//   在 TLS ClientHello 阶段，我们只能看到 SNI（Host），
		//   无法看到 Path/Headers 等信息。
		//   因此，同一个 SNI 的所有路由必须使用相同的 TLS 配置，
		//   否则 ClientHello 阶段无法决定用哪个证书。
		tlsOptionsName := nmqtls.DefaultTLSConfigName
		// 条件: len(routerHTTPConfig.TLS.Options) > 0 && routerHTTPConfig.TLS.Options != nmqtls.DefaultTLSConfigName
		// 成立: 用户显式配置了非默认的 TLS Options，如 tlsOptions: myCustomTLS
		// 不成立: 没有配置 TLS Options 或配置的就是 "default"，使用默认值
		// 示例:
		//   tls:
		//     options: myCustomTLS    → 条件成立
		//   tls:
		//     options: default       → 条件不成立
		//   tls: {}                   → 条件不成立
		if len(routerHTTPConfig.TLS.Options) > 0 && routerHTTPConfig.TLS.Options != nmqtls.DefaultTLSConfigName {
			tlsOptionsName = provider.GetQualifiedName(ctxRouter, routerHTTPConfig.TLS.Options)
		}

		// 从路由规则中解析域名（Host 匹配器中的域名）。
		// 例如: rule: Host("foo.com") && PathPrefix("/bar") → domains = ["foo.com"]
		domains, err := httpmuxer.ParseDomains(routerHTTPConfig.Rule)
		// 如果规则解析失败:
		// - 条件成立: 路由规则语法错误，无法提取域名
		// - 处理: 将错误标记到路由器配置上，跳过该路由
		// - 严重程度: true（这是一个严重错误，路由器将进入错误状态）
		if err != nil {
			routerErr := fmt.Errorf("invalid rule %s, error: %w", routerHTTPConfig.Rule, err)
			routerHTTPConfig.AddError(routerErr, true)
			m.log.Error("add http rule failed", zap.Error(routerErr))
			continue
		}

		// 条件: len(domains) == 0
		// 成立: 规则中没有 Host() 匹配器，例如 rule: PathPrefix("/foo")
		//       这种路由器无法在 ClientHello 阶段通过 SNI 精确匹配，
		//       只能作为兜底的 catch-all TLS 配置
		// 不成立: 规则中有 Host() 匹配器，可以为每个域名精确注册 TLS 配置
		if len(domains) == 0 {
			// 注册通配符 "*" 的 TLS 配置作为兜底。
			// 当请求的 SNI 不匹配任何已注册的域名时，使用 defaultTLSConf。
			//
			// 关键概念——为什么没有 Host 规则的路由器不应当指定 TLS Options:
			//   TLS ClientHello 阶段能获取的唯一路由信息是 SNI（域名）。
			//   如果一个路由器没有 Host 规则，无法将请求映射到具体域名，
			//   也就无法在多个 TLS 配置中选择。因此:
			//
			//   conf1: (只有无 Host 的路由器)
			//     httpRouter1:
			//       rule: PathPrefix("/foo")
			//     # 请求无论从哪来，都使用默认 TLS 配置（Host(*) 兜底）
			//
			//   conf2: (混合场景)
			//     httpRouter1:
			//       rule: PathPrefix("/foo")
			//     httpRouter2:
			//       rule: Host("foo.com") && PathPrefix("/bar")
			//       tlsoptions: myTLSOptions
			//     # 当请求 "/foo" 到达时:
			//     #   - 如果 SNI=foo.com → 使用 myTLSOptions（匹配了 httpRouter2 的 Host）
			//     #   - 如果 SNI 不等于 foo.com → 使用默认 TLS 配置
			//     # 注意: httpRouter2 虽然不会路由该请求，但其 TLS 配置仍会影响 TLS 握手
			router.AddHTTPTLSConfig("*", defaultTLSConf, nmqtls.DefaultTLSConfigName)

			// 条件: tlsOptionsName != nmqtls.DefaultTLSConfigName
			// 成立: 用户给一个没有 Host 规则的路由器指定了非默认的 TLS Options
			//       这是无效配置：没有域名就没有映射 TLS Options 的依据
			// 处理: 记录错误，标记路由器配置错误（非致命，不影响其他路由器）
			// 示例错误配置:
			//   httpRouter:
			//     rule: PathPrefix("/api")     # 没有 Host() 规则
			//     tls:
			//       options: myCustomTLS       # 这个 TLS Options 无法应用！
			if tlsOptionsName != nmqtls.DefaultTLSConfigName {
				m.log.Error("no domain found in rule, the TLS option cannot be applied", zap.String(routerHTTPConfig.Rule, tlsOptionsName))
				routerHTTPConfig.AddError(fmt.Errorf("no domain found in rule %v, the TLS option %s cannot be applied", routerHTTPConfig.Rule, tlsOptionsName), false)
			}
		}

		// 条件: len(domains) > 0 && routerHTTPConfig.TLS.ResolvedOptions != tlsOptionsName
		// 成立: 存在域名，但解析后的 TLS Options 名称与期望的不一致
		//       说明同一 EntryPoint 下，同一 Host 的多个路由器配置了冲突的 TLS Options
		// 处理: 添加错误信息，路由器将使用默认 TLS 配置
		// 示例冲突配置:
		//   router1: rule: Host("foo.com") && PathPrefix("/a")  tlsoptions: optA
		//   router2: rule: Host("foo.com") && PathPrefix("/b")  tlsoptions: optB
		//   # 冲突！foo.com 只能有一个 TLS 配置
		if len(domains) > 0 && routerHTTPConfig.TLS.ResolvedOptions != tlsOptionsName {
			routerHTTPConfig.AddError(errors.New("router's TLSOptions configuration is conflicting with other routers on the same entrypoint and host, default TLS options will be used instead"), false)
		}

		// 获取该路由器解析后的实际 TLS 配置对象。
		// 注意：即使获取失败也继续 —— tlsConf 为 nil 是一个信号，
		// 表示该域名的 TLS 配置不可用，Router 会拒绝该域名的 TLS 连接。
		tlsConf, tlsConfErr := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, routerHTTPConfig.TLS.ResolvedOptions)
		// 如果获取 TLS 配置失败:
		// - 条件成立: TLS 管理器中没有对应的证书配置
		// - 处理: 仅记录日志，不调用 AddError（因为在 buildRouterHandler 阶段已经报过错）
		// - 影响: tlsConf 为 nil，后续会为每个域名插入 brokenTLS 处理器
		if tlsConfErr != nil {
			m.log.Error("failed to get tls config", zap.Error(tlsConfErr))
		}

		// 为每个解析出的域名注册 TLS 配置映射
		for _, domain := range domains {
			// 条件: tlsConf == nil
			// 成立: 该域名的 TLS 配置获取失败（证书不存在或无效）
			// 处理: 注册一个 nil TLS 配置，Router 会插入 brokenTLSRouter，
			//       强制拒绝该域名的所有 TLS 连接，防止不安全降级
			//       然后 continue 跳到下一个域名
			// 不成立: TLS 配置有效，正常注册域名 → TLS 配置映射
			if tlsConf == nil {
				m.log.Error("failed to get tls config", zap.Error(tlsConfErr))
				router.AddHTTPTLSConfig(domain, nil, "")
				continue
			}

			m.log.Info("add http handler", zap.String("domain", domain), zap.String("tls options", routerHTTPConfig.TLS.ResolvedOptions))
			router.AddHTTPTLSConfig(domain, tlsConf, routerHTTPConfig.TLS.ResolvedOptions)
		}

	}

	// ---- 第二阶段：注册 HTTPS 处理器（TLS 解密后的 HTTP 流量处理器） ----
	// defaultTLSConf 可能为 nil —— 这是合法的，Router 会内部处理
	router.SetHTTPSHandler(handlerHTTPS, defaultTLSConf)

	// ---- 第三阶段：注册 TCP 路由处理器 ----
	m.addTCPHandlers(ctx, configs, router)

	return router, nil
}

// addTCPHandlers 将所有 TCP 路由注册到 Router 中。
//
// 根据路由的 TLS 配置，将路由注册到不同的 muxer:
//   - TLS == nil               → muxerTCP    (纯 TCP，无 TLS)
//   - TLS.Passthrough == true  → muxerTCPTLS (TLS 透传，不解密)
//   - 其他 TLS 配置             → muxerTCPTLS (TLS 终止，由 NMQ 解密)
//
// 参数:
//   - ctx:    上下文
//   - configs: TCP 路由配置映射
//   - router: 目标 Router 实例
//
// 使用示例:
//
//	tcpConfigs := map[string]*runtimecfg.TCPRouterInfo{
//	    "mysql-router@file": {Rule: "HostSNI(`mysql.example.com`)", TLS: &TLSConfig{...}},
//	    "ssh-router@file":   {Rule: "HostSNI(`*`)", TLS: nil},
//	}
//	m.addTCPHandlers(ctx, tcpConfigs, router)
func (m *Manager) addTCPHandlers(ctx context.Context, configs map[string]*runtimecfg.TCPRouterInfo, router *Router) {
	for routerName, routerConfig := range configs {
		m.log.Info("add tcp handler", zap.String("router name", routerName))
		// 将 routerName 附加到 context 上，供后续 provider.GetQualifiedName 使用
		cxtRouter := provider.AddInContext(ctx, routerName)

		// ---- 第1步：自动计算优先级（如果用户未指定） ----
		// 条件: routerConfig.Priority == 0
		// 成立: 用户没有在配置中显式指定 Priority
		//       例如: router: { rule: "HostSNI(`foo.com`)" }  # 没有 priority 字段
		// 处理: 根据规则自动推断优先级
		// 不成立: 用户已指定优先级，保留用户设置值
		//       例如: router: { rule: "HostSNI(`foo.com`)", priority: 100 }
		if routerConfig.Priority == 0 {
			routerConfig.Priority = tcpmuxer.GetRulePriority(routerConfig.Rule)
		}

		// ---- 第2步：校验 Service 必填 ----
		// 条件: routerConfig.Service == ""
		// 成立: 路由器没有指定后端 Service，无法转发流量
		// 处理: 标记严重错误 (true)，跳过该路由
		// 不成立: Service 已指定，继续后续校验
		// 示例正确配置:
		//   tcpRouter:
		//     rule: "HostSNI(`mysql.example.com`)"
		//     service: my-mysql-service    # ← 必须有
		if routerConfig.Service == "" {
			err := errors.New("the service is missing on the router")
			routerConfig.AddError(err, true)
			m.log.Error("add tcp handler failed", zap.Error(err))
			continue
		}

		// ---- 第3步：校验 Rule 必填 ----
		// 条件: routerConfig.Rule == ""
		// 成立: 路由器没有匹配规则，无法判断哪些请求应该路由到此服务
		// 处理: 标记严重错误 (true)，跳过该路由
		// 不成立: 规则已指定，继续解析
		if routerConfig.Rule == "" {
			err := errors.New("router has no rule")
			routerConfig.AddError(err, true)
			m.log.Error("add tcp rule failed", zap.Error(err))
			continue
		}

		// ---- 第4步：解析 HostSNI 规则 ----
		// 从规则中提取 HostSNI 匹配的域名列表。
		// 例如: rule: "HostSNI(`foo.com`) || HostSNI(`bar.com`)" → domains = ["foo.com", "bar.com"]
		domains, err := tcpmuxer.ParseHostSNI(routerConfig.Rule)
		// 如果规则解析失败:
		// - 条件成立: 规则语法有误，无法解析 HostSNI
		// - 处理: 标记严重错误，跳过
		// - 示例错误规则: "HostSNI("  # 括号不匹配
		if err != nil {
			routerErr := fmt.Errorf("invalid rule: %q , %w", routerConfig.Rule, err)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(err))
			continue
		}

		// ---- 第5步：HostSNI 规则必须配合 TLS 使用 ----
		// 条件: len(domains) > 0 && routerConfig.TLS == nil && domains[0] != "*"
		// 三个子条件:
		//   ① len(domains) > 0:      规则中有 HostSNI 匹配器（与 SNI 相关）
		//   ② routerConfig.TLS == nil: 路由器没有配置 TLS
		//   ③ domains[0] != "*":     不是通配符 HostSNI(*)
		//
		// 成立: 有不带 TLS 的 HostSNI 规则（通配符除外）
		//       HostSNI 用于匹配 TLS ClientHello 中的 SNI，没有 TLS 就没有 SNI 可匹配
		//       因此这是一个配置错误
		// 不成立的情况:
		//   - 没有 HostSNI 规则：纯 TCP 路由不需要 TLS
		//   - 有 TLS 配置：HostSNI + TLS 是合法组合
		//   - HostSNI(*) 通配符：兜底规则允许不带 TLS（匹配所有 SNI，包括无 SNI 的连接）
		//
		// 示例错误配置:
		//   tcpRouter:
		//     rule: "HostSNI(`mysql.example.com`)"  # 使用了 HostSNI
		//     # 但没有 tls: {} 配置块 → 错误！
		//
		// 示例合法配置（通配符例外）:
		//   tcpRouter:
		//     rule: "HostSNI(`*`)"  # 通配符允许不带 TLS
		//     service: fallback-service
		if len(domains) > 0 && routerConfig.TLS == nil && domains[0] != "*" {
			routerErr := fmt.Errorf("invalid rule: %q , has HostSNI matcher, but no TLS on router", routerConfig.Rule)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(routerErr))
			continue
		}

		// ---- 第6步：优先级上限检查 ----
		// 条件: routerConfig.Priority > maxUserPriority && !strings.HasSuffix(routerName, "@internal")
		// 两个子条件:
		//   ① routerConfig.Priority > maxUserPriority: 优先级超过用户上限
		//   ② !strings.HasSuffix(routerName, "@internal"): 不是内部路由器
		//
		// 成立: 用户定义的路由器使用了保留的高优先级值
		//       高优先级区间 (maxUserPriority, MaxInt] 保留给内部路由器
		//       防止用户路由器抢占内部路由器的位置
		// 不成立的情况:
		//   - 优先级在合法范围内
		//   - 是内部路由器（@internal 后缀），可以使用高优先级
		//
		// 示例:
		//   my-router@file    priority=2147482647 → 错误（超过 maxUserPriority）
		//   my-router@internal priority=2147483647 → 合法（内部路由器）
		if routerConfig.Priority > maxUserPriority && !strings.HasSuffix(routerName, "@internal") {
			routerErr := fmt.Errorf("the router priority %d exceeds the max user-defined priority %d", routerConfig.Priority, maxUserPriority)
			routerConfig.AddError(routerErr, true)
			m.log.Error("add tcp rule failed", zap.Error(routerErr))
			continue
		}

		// ---- 第7步：构建 TCP 处理器（针对非 TLS 终止场景） ----
		// 条件: routerConfig.TLS == nil || routerConfig.TLS.Passthrough
		// 两个子条件（满足任一即成立）:
		//   ① routerConfig.TLS == nil: 纯 TCP 路由，没有 TLS
		//   ② routerConfig.TLS.Passthrough: TLS 透传，不需要 NMQ 解密
		//
		// 成立: 先构建 handler，因为后续两个分支都需要
		// 不成立: TLS 终止模式，handler 在后面包装 TLSHandler 时再构建
		//
		// handler 构建流程:
		//   中间件链(middleware1 → middleware2) → 负载均衡器 → 后端服务
		var handler tcp.Handler
		if routerConfig.TLS == nil || routerConfig.TLS.Passthrough {
			handler, err = m.buildTCPHandler(cxtRouter, routerConfig)
			// 如果构建失败:
			// - 条件成立: Service 不存在或中间件链构建失败
			// - 处理: 标记严重错误，跳过该路由
			if err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp handler failed", zap.Error(err))
				continue
			}
		}

		// ---- 第8步：路由注册——纯 TCP 无 TLS ----
		// 条件: routerConfig.TLS == nil
		// 成立: 纯 TCP 路由，不涉及 TLS
		// 处理: 注册到 muxerTCP，然后 continue 跳到下一个路由器
		// 不成立: 有 TLS 配置，进入后续分支
		//
		// muxerTCP 路由匹配示例:
		//   rule: "HostSNI(`*`)"  → 匹配所有 TCP 连接
		//   rule: "ClientIP(`10.0.0.1`)" → 匹配来自指定 IP 的连接
		if routerConfig.TLS == nil {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))

			// 将路由添加到 muxerTCP：纯 TCP 连接，不涉及 TLS 握手
			if err = router.muxerTCP.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp rule failed", zap.Error(err))
			}
			continue
		}

		// ---- 第9步：路由注册——TLS 透传 ----
		// 条件: routerConfig.TLS.Passthrough
		// 成立: TLS Passthrough 模式，NMQ 不解密 TLS，直接将加密流量透传到后端
		//       后端服务负责 TLS 握手和解密
		// 处理: 注册到 muxerTCPTLS，然后 continue
		// 不成立: TLS 终止模式，NMQ 需要解密 TLS，进入第10步
		//
		// Passthrough 使用场景:
		//   后端服务有自己的 TLS 证书（如企业内部服务），
		//   不需要在 NMQ 层做 TLS 终止
		//
		// 配置示例:
		//   tcpRouter:
		//     rule: "HostSNI(`internal.example.com`)"
		//     tls:
		//       passthrough: true    # NMQ 不做 TLS 解密
		//     service: internal-svc
		if routerConfig.TLS.Passthrough {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))

			// 注册到 muxerTCPTLS：TLS 连接会匹配到此路由，但 TLS 流量原样透传
			if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp tls rule failed")
			}
			continue
		}

		// ---- 第10步：域名 ASCII 校验（TLS 终止模式） ----
		// 到达这里 = TLS 终止模式（非 nil 且非 Passthrough）
		//
		// 遍历所有解析出的域名，检查是否为纯 ASCII 字符。
		// 为什么只允许 ASCII：SNI 在 TLS ClientHello 中以 ASCII 编码传输，
		// 非 ASCII 字符会导致 TLS 握手问题。
		for _, domain := range domains {
			m.log.Info("Add route for", zap.String("rule", routerConfig.Rule))
			// 条件: muxer.IsASCII(domain)
			// 成立: 域名是纯 ASCII 字符，合法
			//       处理: continue 检查下一个域名
			// 不成立: 域名包含非 ASCII 字符（如中文域名等国际字符）
			//       处理: 记录错误，标记路由器
			//       注意: 这里没有 continue 跳过整个路由，
			//            只是标记错误但路由仍可能被添加
			if muxer.IsASCII(domain) {
				continue
			}

			// 非 ASCII 域名 → 标记错误
			// 例如: domain = "中文域名.com" → 非法
			asciiError := fmt.Errorf("invalid domain name value %q, non-ASCUU characters are not allowed", domain)
			routerConfig.AddError(asciiError, true)
			m.log.Error("add tcp tls rule failed", zap.Error(asciiError))
		}

		// ---- 第11步：确定 TLS Options 名称 ----
		// 流程:
		//   1. 从配置中读取 tlsOptionsName
		//   2. 如果为空，使用默认值 "default"
		//   3. 如果不是默认值，转为完整限定名（带 provider 前缀）
		tlsOptionsName := routerConfig.TLS.Options

		// 条件: len(tlsOptionsName) == 0
		// 成立: 用户没有指定 TLS Options
		//       例如: tls: {}  # options 为空
		// 处理: 回退到默认 TLS 配置名称
		// 不成立: 用户指定了 TLS Options，保持用户设置
		if len(tlsOptionsName) == 0 {
			tlsOptionsName = nmqtls.DefaultTLSConfigName
		}

		// 条件: tlsOptionsName != nmqtls.DefaultTLSConfigName
		// 成立: 用户指定了非默认的 TLS Options
		//       例如: tlsOptionsName = "myCustomTLS"
		// 处理: 转为完整限定名 "myCustomTLS@providerName"
		// 不成立: 使用的是默认 TLS 配置，名称就是 "default"，无需限定
		if tlsOptionsName != nmqtls.DefaultTLSConfigName {
			tlsOptionsName = provider.GetQualifiedName(cxtRouter, tlsOptionsName)
		}

		// ---- 第12步：获取 TLS 配置并处理获取失败 ----
		tlsConf, err := m.tlsManager.Get(nmqtls.DefaultTLSStoreName, tlsOptionsName)
		// 如果获取 TLS 配置失败（证书不存在或无效）:
		// - 条件成立: TLS 管理器中没有对应证书
		// - 处理:
		//    1. 标记路由器严重错误
		//    2. 但仍然注册路由，使用 brokenTLSRouter 作为处理器
		//       brokenTLSRouter 会拒绝所有 TLS 连接，防止降级攻击
		//    3. continue 跳过正常的 TLS 处理器构建
		// - 不成立: TLS 配置有效，继续构建 TLS 终止处理器
		if err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.String(routerConfig.Rule, tlsOptionsName), zap.Error(err))

			// 注册 brokenTLSRouter：即使 TLS 配置获取失败，也注册路由
			// 目的: 确保 muxer 能匹配到该路由并返回合理的错误（拒绝连接），
			//       而不是 fallback 到其他可能不安全的路由
			if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), &brokenTLSRouter{}); err != nil {
				routerConfig.AddError(err, true)
				m.log.Error("add tcp tls rule failed", zap.Error(err))
			}

			continue
		}

		// ---- 第13步：构建 TLS 终止处理器 ----
		// 关于同一 HostSNI 不同 TLS Options 的说明:
		//   理论上支持同一 HostSNI 配置不同 TLS Options:
		//     router1:
		//       rule: HostSNI(foo.com) && ClientIP(IP1)  tlsOption: tlsOne
		//     router2:
		//       rule: HostSNI(foo.com) && ClientIP(IP2)  tlsOption: tlsTwo
		//   这仅在 muxer 能在 TLS 握手之前完成路由决策时才合法
		//   （即在告诉客户端使用哪个证书之前就知道最终路由）。
		//   目前支持的匹配器（HostSNI、ClientIP）都能在 TLS 握手前完成匹配，
		//   所以这种配置是合法的。
		//   如果未来添加了需要解密后才能匹配的规则（如 Path 匹配），
		//   则需要像 HTTPS 一样，禁止同一 HostSNI 使用不同 TLS 配置。
		handler, err = m.buildTCPHandler(cxtRouter, routerConfig)
		// 如果构建失败:
		// - 条件成立: Service 不存在或中间件构建失败
		// - 处理: 标记严重错误，跳过该路由
		if err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.Error(err))
			continue
		}

		// 将 TCP 处理器包装在 TLSHandler 中。
		// 请求流程: 客户端 → TLS握手(NMQ解密) → handler(中间件链 → 服务) → 后端
		handler = &tcp.TLSHandler{
			Next:           handler,        // TLS 解密后的 TCP 处理器
			Config:         tlsConf,        // 用于 TLS 握手的证书配置
			TLSOptionsName: tlsOptionsName, // TLS 选项名称（用于 HTTP/3 匹配）
		}

		m.log.Debug("Add route for", zap.String("rule", routerConfig.Rule), zap.String("tlsOptionsName", tlsOptionsName))

		// 注册到 muxerTCPTLS：TLS 连接到达后，先匹配路由，NMQ 做 TLS 解密，
		// 然后将明文流量传给 handler 处理
		if err = router.muxerTCPTLS.AddRoute(routerConfig.Rule, routerConfig.RuleSyntax, routerConfig.Priority, providerName(routerName), handler); err != nil {
			routerConfig.AddError(err, true)
			m.log.Error("add tcp tls rule failed", zap.Error(err))
			continue
		}

	}
}

// buildTCPHandler 构建 TCP 处理器链。
//
// 处理器链结构（从外到内）:
//
//	连接 → 中间件1 → 中间件2 → ... → 中间件N → 负载均衡 → 后端服务
//
// 参数:
//   - ctx:    上下文（携带 router 名称信息）
//   - router: TCP 路由器配置（包含 Service 和 Middlewares）
//
// 返回值: 构建好的 TCP 处理器链
//
// 使用示例:
//
//	// 配置如下:
//	//   tcpRouter:
//	//     middlewares: [rate-limiter, ip-whitelist]
//	//     service: my-backend-service
//	//
//	// 生成的处理器链:
//	//   rate-limiter → ip-whitelist → load-balancer(my-backend-service)
func (m *Manager) buildTCPHandler(ctx context.Context, router *runtimecfg.TCPRouterInfo) (tcp.Handler, error) {
	// 将中间件名称转为完整限定名
	// 例如: "rate-limiter" → "rate-limiter@file"
	var qualifiedName []string
	for _, name := range router.Middlewares {
		qualifiedName = append(qualifiedName, provider.GetQualifiedName(ctx, name))
	}

	router.Middlewares = qualifiedName

	// 条件: router.Service == ""
	// 成立: 路由器没有配置 Service
	//       这理论上不会发生（addTCPHandlers 中已校验），但作为防御性编程再次检查
	// 处理: 返回错误
	if router.Service == "" {
		return nil, errors.New("the service is missing on the router")
	}

	// 构建后端服务处理器（包含负载均衡逻辑）
	sHandler, err := m.serviceManager.BuildTCP(ctx, router.Service)
	// 如果构建失败:
	// - 条件成立: Service 不存在、后端服务器不可达等
	// - 处理: 包装错误信息并返回
	if err != nil {
		return nil, fmt.Errorf("build tcp handler failed %w", err)
	}

	// 构建中间件链
	mHandler := m.middlewareBuilder.BuildChain(ctx, router.Middlewares)

	// 将中间件链与后端处理器组合:
	//   Extend 将 mHandler 的构造器追加到新链后面
	//   Then 将最终的后端处理器作为链的终点
	// 结果: mHandler 中间件 → sHandler 后端服务
	return tcp.NewChain().Extend(*mHandler).Then(sHandler)
}

// providerName 从路由器名称中提取 provider 名称。
//
// 路由器名称格式: "routerName@providerName"
//
//	例如: "my-tcp-router@file" → provider = "file"
//	      "my-tcp-router@docker" → provider = "docker"
//	      "my-tcp-router"        → provider = ""（没有 @ 分隔符）
//
// 参数:
//   - routerName: 路由器名称，格式为 "name@provider"
//
// 返回值: provider 名称（@ 后面的部分），如果没有 @ 则返回空字符串
//
// 使用示例:
//
//	providerName("my-router@file")   → "file"
//	providerName("my-router@docker") → "docker"
//	providerName("my-router")        → ""
func providerName(routerName string) string {
	parts := strings.Split(routerName, "@")
	// 条件: len(parts) == 2
	// 成立: 路由器名称包含 "@" 分隔符，格式为 "name@provider"
	//       返回 provider 名称（@ 后面的部分）
	// 不成立: 路由器名称不包含 "@" 或包含多个 "@"
	//       返回空字符串，表示 provider 未知
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
