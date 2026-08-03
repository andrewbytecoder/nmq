// Package tcp 提供 TCP 服务的管理和构建能力。
//
// Manager 是 TCP 服务 handler 的工厂。它从运行时配置中读取所有已注册的 TCP 服务定义，
// 按需将其构建为可执行的 tcp.Handler。支持两种服务类型：
//
//   - LoadBalancer：直接配置后端服务器列表，创建加权轮询负载均衡器
//   - Weighted：组合多个子服务（可以是 LoadBalancer 或其他 Weighted），
//     形成层级化的加权轮询树
//
// 每个 Manager 管理一组健康检查器（healthCheckers），在服务构建完成后
// 通过 LaunchHealthCheck 统一启动。
package tcp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"net"
	"slices"
	"time"

	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	"github.com/andrewbytecoder/nmq/plugins/proxy/healthcheck"
	"github.com/andrewbytecoder/nmq/plugins/proxy/server/provider"
	"github.com/andrewbytecoder/nmq/plugins/proxy/tcp"
	"go.uber.org/zap"
)

// Manager TCP 服务处理器工厂。
//
// 职责：
//   - 持有所有 TCP 服务的运行时配置（configs）
//   - 按需构建 tcp.Handler 实例（BuildTCP），支持递归构建 Weighted 子树
//   - 管理 TCP 层面的健康检查器（healthCheckers），在 LaunchHealthCheck 中统一启动
//
// 并发安全：
//   - configs map 在初始化后只读，运行时不会被修改，无需加锁
//   - healthCheckers map 在 BuildTCP 阶段写入（单线程配置构建），
//     在 LaunchHealthCheck 阶段只读，无需加锁
//   - rand 实例非并发安全，但只在 BuildTCP（配置构建阶段）中使用，调用方保证串行
type Manager struct {
	log            *zap.Logger                                     // 日志记录器
	dialerManager  *tcp.DialerManager                              // 连接拨号器管理器，用于创建到后端服务器的 TCP 连接
	configs        map[string]*runtimecfg.TCPServiceInfo           // 所有 TCP 服务的运行时配置，key 为全限定名（name@provider）
	rand           *rand.Rand                                      // 随机数生成器，用于初始时打乱服务器/子服务顺序，避免"热启动"时所有请求打到同一后端
	healthCheckers map[string]*healthcheck.ServiceTCPHealthChecker // 健康检查器实例，key 为服务全限定名，在 LaunchHealthCheck 中统一启动
}

// NewManager 创建 Manager 实例。
//
// 参数：
//
//	log:           日志记录器
//	conf:          运行时配置（从中提取 TCPServices map）
//	dialerManager: TCP 连接拨号器管理器
//
// 初始化细节：
//   - healthCheckers 初始为空 map，后续在 BuildTCP 中逐步填充
//   - configs 直接引用 conf.TCPServices（非拷贝），因此 Manager 的生命周期
//     应不短于 conf 的生命周期
//   - rand 使用当前纳秒时间戳作为种子，确保每次构建时的打乱顺序不同
func NewManager(log *zap.Logger, conf *runtimecfg.Configuration, dialerManager *tcp.DialerManager) *Manager {
	return &Manager{
		log:            log,
		dialerManager:  dialerManager,
		healthCheckers: make(map[string]*healthcheck.ServiceTCPHealthChecker),
		configs:        conf.TCPServices,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// BuildTCP 根据服务名构建 tcp.Handler。
//
// 这是 Manager 的核心方法，支持递归构建：当遇到 Weighted 类型的服务时，
// 会递归调用自身来构建子服务的 handler，形成树状负载均衡结构。
//
// 参数：
//
//	rootCtx:     根上下文，携带 provider 信息（用于限定名解析）
//	serviceName: 要构建的服务名称（可以是短名或已包含 @provider 的全限定名）
//
// 构建流程（两种服务类型）：
//
// ┌─ LoadBalancer 类型 ──────────────────────────────────────────┐
// │                                                              │
// │  1. 检查废弃配置项（TerminationDelay、ProxyProtocol），打印警告 │
// │  2. 补全 ServersTransport 的全限定名                           │
// │  3. 遍历 Servers 列表：                                       │
// │     a. 校验地址格式（net.SplitHostPort）                       │
// │     b. 创建到后端的 TCP dialer                                │
// │     c. 创建 TCP 代理 handler                                  │
// │     d. 加入负载均衡器（默认权重=1，初始状态=UP）                  │
// │     e. 收集健康检查目标                                        │
// │  4. 如果配置了健康检查 → 创建主动健康检查器（TCP 层面）          │
// │  5. 返回负载均衡器 handler                                     │
// │                                                              │
// └──────────────────────────────────────────────────────────────┘
//
// ┌─ Weighted 类型 ───────────────────────────────────────────────┐
// │                                                              │
// │  1. 创建空的 WRRLoadBalancer                                   │
// │  2. 遍历子服务列表：                                           │
// │     a. 递归调用 BuildTCP 构建子服务 handler（可能再展开）        │
// │     b. 将子 handler 加入负载均衡器（携带用户配置的权���）          │
// │     c. 如果配置了健康检查：                                     │
// │        - 检查子 handler 是否实现了 StatusUpdater 接口           │
// │        - 注册状态变化回调：子服务状态变化 → 通知父负载均衡器      │
// │  3. 返回负载均衡器 handler                                     │
// │                                                              │
// └──────────────────────────────────────────────────────────────┘
//
// 递归安全性：
//   - for range 遍历的是有限子服务列表，正常配置下递归必然终止
//   - 如果配置中存在循环引用（A→B→A），会导致栈溢出，
//     需要配置层保证服务之间形成 DAG（有向无环图）
func (m *Manager) BuildTCP(rootCtx context.Context, serviceName string) (tcp.Handler, error) {
	// ── 第1步：解析服务的全限定名 ──
	// provider.GetQualifiedName 从 context 中提取 provider 名称，
	// 将短名（如 "my-service"）补全为全限定名（如 "my-service@file"）。
	// 如果 serviceName 本身已包含 @，则直接返回原值。
	serviceQualifiedName := provider.GetQualifiedName(rootCtx, serviceName)

	m.log.Info("build qualified name", zap.String("serviceQualifiedName", serviceQualifiedName))

	// ── 第2步：将当前服务的 provider 信息注入 context ──
	// provider.AddInContext 从 serviceQualifiedName 中提取 @ 后面的 provider 名称，
	// 写入 context。这样后续递归构建子服务时，子服务的短名可以被正确补全。
	// 例如：当前服务 "parent@file" → ctx 中注入 "file"，
	// 子服务短名 "child" → GetQualifiedName 补全为 "child@file"
	ctx := provider.AddInContext(rootCtx, serviceQualifiedName)

	// ── 第3步：从配置 map 中查找服务定义 ──
	conf, ok := m.configs[serviceQualifiedName]
	// if !ok 为 true：全限定名在 configs 中不存在
	//   可能原因：配置中未定义该服务，或 provider 名称不匹配
	//   → 返回错误，上层会终止整个构建流程
	if !ok {
		return nil, fmt.Errorf("the service %q does not exist", serviceQualifiedName)
	}

	// ── 第4步：互斥检查 ──
	// LoadBalancer 和 Weighted 是互斥的两种服务类型，一个服务定义中
	// 只能选择其一。如果两者同时非 nil，说明配置有误。
	// if conf.LoadBalancer != nil && conf.Weighted != nil 为 true：
	//   两种类型同时配置 → 错误，拒绝构建
	//   conf.AddError(err, true)：第二个参数为 true 表示这是一个"严重错误"，
	//   会阻止路由生效
	if conf.LoadBalancer != nil && conf.Weighted != nil {
		err := errors.New("cannot create service: multi-types service not supported, consider declaring two different pieces of service instead")
		conf.AddError(err, true)
		return nil, err
	}

	// ── 第5步：根据服务类型分发构建 ──
	// switch 无表达式（tagless switch），等价于多个 if-else 判断。
	// 由于第4步已确保互斥，最多只有一个 case 会命中。
	switch {
	// ================================================================
	// case 1: LoadBalancer 类型 —— 直接配置后端服务器列表
	// ================================================================
	case conf.LoadBalancer != nil:
		// 创建加权轮询负载均衡器
		// conf.LoadBalancer.HealthCheck != nil → wantsHealthCheck 参数：
		//   true:  允许子节点注册状态变化回调（通过 RegisterStatusUpdater）
		//   false: 不启用健康检查，RegisterStatusUpdater 会拒绝注册
		loadBalancer := tcp.NewWRRLoadBalancer(m.log, conf.LoadBalancer.HealthCheck != nil)

		// ── 废弃配置项警告 ──
		// TerminationDelay 和 ProxyProtocol 已废弃，功能已迁移到 ServersTransport 配置中。
		// 这里仅打印警告日志，不阻止构建，保持向后兼容。

		// if conf.LoadBalancer.TerminationDelay != nil 为 true：
		//   用户还在使用已废弃的 TerminationDelay 配置项
		//   → 打印警告，建议迁移到 ServersTransport
		if conf.LoadBalancer.TerminationDelay != nil {
			m.log.Warn("Service load balancer uses `TerminationDelay`, but this option is deprecated, please use ServersTransport configuration instead.",
				zap.String("serviceName", serviceName))
		}

		// if conf.LoadBalancer.ProxyProtocol != nil 为 true：
		//   用户还在使用已废弃的 ProxyProtocol 配置项
		//   → 打印警告，建议迁移到 ServersTransport
		if conf.LoadBalancer.ProxyProtocol != nil {
			m.log.Warn("Service load balancer uses `ProxyProtocol`, but this option is deprecated, please use ServersTransport configuration instead.",
				zap.String("serviceName", serviceName))
		}

		// ── 补全 ServersTransport 的全限定名 ──
		// if len(conf.LoadBalancer.ServersTransport) > 0 为 true：
		//   用户配置了自定义的 ServersTransport 名称（短名）
		//   → 使用当前 context 中的 provider 信息补全为全限定名
		//   例如：ctx 中 provider="file"，短名 "myTransport" → "myTransport@file"
		//
		// if len(...) > 0 为 false（空字符串）：
		//   使用默认的 ServersTransport，无需处理
		if len(conf.LoadBalancer.ServersTransport) > 0 {
			conf.LoadBalancer.ServersTransport = provider.GetQualifiedName(ctx, conf.LoadBalancer.ServersTransport)
		}

		// ── 准备健康检查目标收集器 ──
		// 使用 map 去重：同一个 server.Address 只会保留一份健康检查配置
		// 预分配容量 = 服务器数量，避免 map 扩容
		uniqHealthCheckTargets := make(map[string]healthcheck.TCPHealthCheckTarget, len(conf.LoadBalancer.Servers))

		// ── 遍历后端服务器列表 ──
		// shuffle 将服务器列表随机打乱，目的是避免"惊群效应"：
		//   不 shuffle：所有 Manager 实例按相同顺序排列服务器，
		//     轮询的起始 index 都是 0，初始请求全打到第一个服务器
		//   shuffle 后：不同实例有不同的排列顺序，初始请求分散到不同服务器
		for index, server := range shuffle(conf.LoadBalancer.Servers, m.rand) {
			m.log.Info("LoadBalance", zap.Int("server index", index), zap.String("serverAddress", server.Address))

			// ── 校验地址格式 ──
			// net.SplitHostPort 将 "host:port" 拆分为 host 和 port 两部分。
			// if err != nil 为 true：地址格式不合法（缺少端口、格式错误等）
			//   → 打印日志并跳过该服务器（continue），不阻止其他服务器的构建
			//
			// if err != nil 为 false：地址格式正确，继续后续步骤
			if _, _, err := net.SplitHostPort(server.Address); err != nil {
				m.log.Info("failed to spilt host port")
				continue
			}

			// ── 创建到后端的 TCP 拨号器 ──
			// dialerManager.Build 根据服务器配置（TLS 设置、传输层选项等）
			// 创建一个 net.Dialer，用于与后端建立 TCP 连接。
			// if err != nil 为 true：拨号器创建失败（通常是 TLS 配置错误）
			//   → 这是致命错误，直接 return，终止整个服务的构建
			dialer, err := m.dialerManager.Build(conf.LoadBalancer, server.TLS)
			if err != nil {
				return nil, err
			}

			// ── 创建 TCP 代理 handler ──
			// tcp.NewProxy 创建一个 TCP 层代理：收到客户端连接后，
			// 使用 dialer 拨号到后端地址，在两个连接之间双向拷贝数据。
			// if err != nil 为 true：代理创建失败
			//   → 记录错误并跳过该服务器（continue），不阻止其他服务器的构建
			handler, err := tcp.NewProxy(m.log, server.Address, dialer)
			if err != nil {
				m.log.Error("Failed to create server")
				continue
			}

			// ── 将代理注册到负载均衡器 ──
			// Add(name, handler, weight)：
			//   name:    服务器地址作为标识（用于健康检查的状态映射）
			//   handler: TCP 代理 handler
			//   weight:  nil → 使用默认权重 1（所有服务器平等轮询）
			loadBalancer.Add(server.Address, handler, nil)

			// ── 初始化服务器状态 ──
			// 新注册的服务器默认标记为 UP（健康可用）。
			// 后续由健康检查器根据探测结果更新此状态。
			conf.UpdateServerStatus(server.Address, runtimecfg.StatusUp)

			// ── 收集健康检查目标 ──
			// 每个服务器的 TCPHealthCheckTarget 包含：
			//   Address: 后端地址
			//   TLS:     TLS 配置（用于健康检查时建立加密连接）
			//   Dialer:  连接拨号器（复用已有的连接配置）
			// 使用 server.Address 作为 map key，同名地址自动去重
			uniqHealthCheckTargets[server.Address] = healthcheck.TCPHealthCheckTarget{
				Address: server.Address,
				TLS:     server.TLS,
				Dialer:  dialer,
			}
			m.log.Info("Create TCP server")
		}

		// ── 创建主动健康检查器 ──
		// if conf.LoadBalancer.HealthCheck != nil 为 true：
		//   用户配置了健康检查 → 创建 TCP 层面的主动健康检查器
		//   将健康检查目标列表（去重后）传递给检查器
		//
		// if conf.LoadBalancer.HealthCheck != nil 为 false：
		//   未配置健康检查 → 跳过，不创建健康检查器
		//
		// 注意：健康检查器的 Launch 在 BuildTCP 完成后由 LaunchHealthCheck 统一调用，
		// 此处只是创建实例并注册到 m.healthCheckers 中
		if conf.LoadBalancer.HealthCheck != nil {
			m.healthCheckers[serviceName] = healthcheck.NewServiceTCPHandlerChecker(
				ctx,
				healthcheck.SetTCPLog(m.log),
				healthcheck.SetTCPConfig(conf.LoadBalancer.HealthCheck),
				healthcheck.SetTCPStatusSetter(loadBalancer),
				healthcheck.SetTCPRuntimeInfo(conf),
				healthcheck.SetTCPHealthCheckTargets(slices.Collect(maps.Values(uniqHealthCheckTargets))),
				healthcheck.SetTCPHealthCheckServiceName(serviceQualifiedName),
			)
		}

		// 返回构建好的负载均衡器 handler
		return loadBalancer, nil

	// ================================================================
	// case 2: Weighted 类型 —— 组合多个子服务，每个子服务有独立权重
	// ================================================================
	case conf.Weighted != nil:
		// 创建加权轮询负载均衡器
		// conf.Weighted.HealthCheck != nil → wantsHealthCheck 参数：
		//   决定是否允许子服务通过 RegisterStatusUpdater 注册状态变化回调
		loadBalancer := tcp.NewWRRLoadBalancer(m.log, conf.Weighted.HealthCheck != nil)

		// ── 递归构建所有子服务 ──
		// shuffle 打乱子服务顺序，避免不同实例的初始 index 相同
		for _, service := range shuffle(conf.Weighted.Services, m.rand) {
			// ── 递归构建子服务 handler ──
			// m.BuildTCP(ctx, service.Name) ：
			//   使用已注入 provider 信息的 ctx，递归构建子服务。
			//   子服务可能是 LoadBalancer 类型（递归终点），也可能是嵌套的 Weighted 类型。
			//   if err != nil 为 true：子服务构建失败
			//     → 终止整个构建，返回错误。采用"全有或全无"策略，
			//        部分子服务构建失败意味着该 Weighted 服务无法正常工作
			handler, err := m.BuildTCP(ctx, service.Name)
			if err != nil {
				m.log.Info("Failed to build TCP handler")
				return nil, err
			}

			// ── 将子服务 handler 注册到负载均衡器 ──
			// Add(name, handler, weight)：
			//   name:    子服务的逻辑名称（区别于 LoadBalancer 用地址作为 name）
			//   handler: 递归构建出的子 handler（可能是 WRRLoadBalancer 或其他）
			//   weight:  用户配置的权重（非 nil，决定了该子服务分配的请求比例）
			loadBalancer.Add(service.Name, handler, service.Weight)

			// ── 健康检查短路 ──
			// if conf.Weighted.HealthCheck == nil 为 true：
			//   未配置健康检查 → 跳过状态回调注册（continue），处理下一个子服务
			//   这是必要的性能优化：如果不需要健康检查，就不做类型断言和回调注册
			//
			// if conf.Weighted.HealthCheck == nil 为 false（已配置健康检查）：
			//   继续执行下面的状态传播逻辑
			if conf.Weighted.HealthCheck == nil {
				continue
			}

			// ── 类型断言：检查子服务是否支持状态传播 ──
			// handler.(healthcheck.StatusUpdater)：
			//   检查子 handler 是否实现了 StatusUpdater 接口。
			//   只有 WRRLoadBalancer（加权轮询负载均衡器）实现了该接口，
			//   因为它内部维护了 status map，支持整体状态的上报。
			//
			// if !ok 为 true：子 handler 未实现 StatusUpdater 接口
			//   → 这意味着该子服务不支持健康检查状态传播，属于配置错误
			//   → 返回错误，终止构建（"全有或全无"策略）
			//
			// if !ok 为 false：断言成功，handler 实现了 StatusUpdater 接口
			//   → 继续注册状态变化回调
			updater, ok := handler.(healthcheck.StatusUpdater)
			if !ok {
				return nil, fmt.Errorf("child service %v of %v not healthcheck.StatusUpdater (%T)", service.Name, serviceName, handler)
			}

			// ── 注册状态变化回调 ──
			// 回调闭包：当子服务的整体健康状态发生变化时（从有可用后端变为全部故障，
			// 或反之），子服务会调用此回调通知父负载均衡器。
			//
			// loadBalancer.SetStatus(ctx, service.Name, up)：
			//   将子服务的状态变化传递给父负载均衡器。
			//   service.Name：父负载均衡器中该子服务的标识
			//   up: true=子服务变为可用，false=子服务变为不可用
			//
			// 这种设计形成了自底向上的状态传播链：
			//   底层服务器 → 子 WRRLoadBalancer → 父 WRRLoadBalancer → ... → 路由器
			//
			// if err != nil 为 true：注册失败（如 wantsHealthCheck=false 时拒绝注册）
			//   → 返回错误，终止构建
			if err = updater.RegisterStatusUpdater(func(up bool) {
				loadBalancer.SetStatus(ctx, service.Name, up)
			}); err != nil {
				return nil, fmt.Errorf("cannot register %v as updater for %v: %w", service.Name, serviceName, err)
			}

			m.log.Info("Child service will update parent on staus change", zap.String("parent", serviceName), zap.String("child", service.Name))
		}

		// 返回构建好的负载均衡器 handler（可能嵌套了多层子负载均衡器）
		return loadBalancer, nil

	// ================================================================
	// default: 既不是 LoadBalancer 也不是 Weighted —— 未定义服务类型
	// ================================================================
	default:
		// 服务配置中既没有 LoadBalancer 也没有 Weighted 字段
		// → 这是一个无效的服务定义
		err := fmt.Errorf("the service %q does not have any type defined", serviceQualifiedName)
		// conf.AddError(err, true)：将错误添加到服务配置的错误列表中
		// 第二个参数 true 表示这是严重错误，会导致该服务的路由不可用
		conf.AddError(err, true)
		return nil, err
	}
}

// LaunchHealthCheck 启动所有已注册的健康检查器。
//
// 每个健康检查器在独立的 goroutine 中异步启动（go hc.Launch(ctx)），
// 互不阻塞。调用方需要确保传入的 ctx 在服务关闭或配置刷新时被取消，
// 以优雅地停止所有健康检查。
//
// 使用场景：
//
//	在所有服务通过 BuildTCP 构建完成后，调用此方法统一启动健康检查。
//	不能在 BuildTCP 中直接启动，因为那时服务可能还未完全构建完成。
//
// 并发安全：
//
//	healthCheckers map 在 BuildTCP 阶段写入完毕后不再修改，
//	LaunchHealthCheck 只读遍历，无需加锁。
func (m *Manager) LaunchHealthCheck(ctx context.Context) {
	// 遍历所有已注册的健康检查器
	for serviceName, hc := range m.healthCheckers {
		m.log.Info("launch ", zap.String("serviceName", serviceName))
		// go hc.Launch(ctx)：在独立 goroutine 中异步启动
		// 不等待 Launch 返回，所有健康检查器并发运行
		// Launch 内部有自己的循环逻辑（ticker + select），在 ctx 取消时退出
		go hc.Launch(ctx)
	}
}

// shuffle 返回一个随机打乱顺序的切片拷贝。
//
// 为什么不直接修改原切片？
//
//	原切片可能被其他 Manager 实例引用（共享配置），修改原切片会影响所有引用方。
//	拷贝一份再打乱，每个 Manager 实例有独立的顺序，互不干扰。
//
// 为什么需要打乱顺序？
//
//	在 BuildTCP 中，服务器列表/子服务列表的顺序决定了 WRRLoadBalancer 的
//	初始遍历顺序。如果不打乱，所有 Manager 实例在启动时都会以相同顺序
//	遍历相同列表，导致index初始值相同 → 第一个请求全部打到同一后端。
//
//	打乱后，每个实例有独立的排列顺序，请求从不同位置开始轮询，
//	实现天然的负载分散。
//
// 泛型参数 T：切片元素类型，通过类型推断自动确定。
//
// 返回值：打乱后的新切片，原切片不变。
func shuffle[T any](values []T, r *rand.Rand) []T {
	// 创建与原切片等长的新切片
	shuffled := make([]T, len(values))
	// copy 将原切片的所有元素复制到新切片中
	copy(shuffled, values)
	// r.Shuffle 使用 Fisher-Yates 洗牌算法，在线性时间内均匀随机排列
	// 回调函数 swap(i, j) 交换 shuffled 中两个位置的元素
	r.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}
