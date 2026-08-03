// Package healthcheck 提供主动和被动两种健康检查机制。
//
// # 主动健康检查 (ServiceHealthChecker)
//
//	定期向后端发起 HTTP 或 gRPC 探测请求，根据响应更新后端状态。
//	健康的后端和故障的后端使用不同的检查频率，故障后端检查更频繁。
//
// # 被动健康检查 (PassiveServiceHealthChecker)
//
//	不主动探测，而是通过包装 HTTP handler 拦截真实请求，
//	统计失败次数做熔断。当失败次数超过阈值时，将后端标记为不健康，
//	经过一个滑动时间窗口后自动恢复。
//
// 两类检查可以共存：主动检查负责恢复状态，被动检查负责快速熔断。
//
// # 使用示例
//
//	// ── 1. 仅主动健康检查 ──
//	shc := healthcheck.NewServiceHealthChecker(
//	    healthcheck.SetServiceName("my-service@file"),
//	    healthcheck.SetLogger(logger),
//	    healthcheck.SetCtx(ctx),
//	    healthcheck.SetBalancer(lb),
//	    healthcheck.SetInfo(serviceInfo),
//	    healthcheck.SetConfig(&dynamic.ServerHealthCheck{
//	        Path:     "/health",
//	        Interval: 10_000_000_000,  // 10s, 单位纳秒
//	        Timeout:  3_000_000_000,   // 3s
//	        Status:   200,             // 只有 200 才认为健康
//	    }),
//	    healthcheck.SetRoundTripper(http.DefaultTransport),
//	    healthcheck.SetTargets(map[string]*url.URL{
//	        "server1": {Host: "10.0.0.1:8080", Scheme: "http"},
//	        "server2": {Host: "10.0.0.2:8080", Scheme: "http"},
//	    }),
//	)
//	go shc.Launch(ctx) // 异步启动，会阻塞在健康检查循环
//
//	// ── 2. 仅被动健康检查（熔断器） ──
//	phc := healthcheck.NewPassiveHealthChecker(
//	    healthcheck.SetPassiveServiceName("my-service@file"),
//	    healthcheck.SetPassiveBalancer(lb),
//	    healthcheck.SetPassiveMaxFailedAttempts(3), // 窗口内失败 3 次就熔断
//	    healthcheck.SetPassiveFailureWindow(types.Duration(10 * time.Second)),
//	)
//	// 用 phc.WrapHandler 包装 proxy handler
//	wrappedHandler := phc.WrapHandler(ctx, proxyHandler, "http://10.0.0.1:8080")
//
//	// ── 3. 主动 + 被动共存 ──
//	phc := healthcheck.NewPassiveHealthChecker(
//	    healthcheck.SetPassiveHasActiveHealthCheck(true), // 关键：恢复由主动检查负责
//	    // ... 其他配置
//	)
//	// 被动负责快速熔断，主动负责探测恢复，各司其职
package healthcheck

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/andrewbytecoder/nmq/internal/config/dynamic"
	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	"github.com/andrewbytecoder/nmq/pkg/options"
	"github.com/andrewbytecoder/nmq/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// StatusSetter 状态设置接口。
//
// 健康检查组件本身不维护后端状态（健康/故障），而是通过该接口通知上层组件（如负载均衡器）。
// 服务（如负载均衡器）应该实现此接口，接收健康检查结果并据此调整流量分配。
//
// 实现示例（伪代码，展示 WRRLoadBalancer 如何实现）：
//
//	func (lb *WRRLoadBalancer) SetStatus(ctx context.Context, childName string, status bool) {
//	    lb.statusMu.Lock()
//	    defer lb.statusMu.Unlock()
//	    if status {
//	        lb.statuses[childName] = StatusUp   // 恢复可用
//	    } else {
//	        lb.statuses[childName] = StatusDown // 标记故障
//	    }
//	}
type StatusSetter interface {
	// SetStatus 设置指定子服务（后端）的健康状态。
	//
	// 参数：
	//   ctx:       上下文（用于日志、取消等）
	//   childName: 子服务的唯一标识，通常是 target 的 name（不是 URL）
	//              LoadBalancer 类型：childName = server.Address（如 "10.0.0.1:8080"）
	//              Weighted 类型：childName = service.Name（如 "backend-service@file"）
	//   status:    true 表示健康可用，false 表示故障不可用
	SetStatus(ctx context.Context, childName string, status bool)
}

// StatusUpdater 状态传播接口。
//
// 当某个服务自身状态发生变化（例如它的所有子节点都挂了），
// 需要将状态变化向上传播给父级服务。实现此接口的服务可以注册一个回调函数，
// 当自身状态变化时触发通知。
//
// 自底向上的状态传播链：
//
//	后端服务器 (health check) → WRRLoadBalancer (StatusUpdater) → 父 WRRLoadBalancer → ... → 路由器
//
// 实现示例（伪代码，展示 WRRLoadBalancer 如何实现）：
//
//	func (lb *WRRLoadBalancer) RegisterStatusUpdater(fn func(up bool)) error {
//	    if !lb.wantsHealthCheck {
//	        return fmt.Errorf("health check not enabled") // 未启用健康检查时拒绝注册
//	    }
//	    lb.statusUpdaters = append(lb.statusUpdaters, fn) // 保存回调
//	    return nil
//	}
//
//	// 当 lb 自身的整体状态改变时（所有子节点都挂了 or 第一个子节点恢复）：
//	func (lb *WRRLoadBalancer) notifyStatusChange(up bool) {
//	    for _, fn := range lb.statusUpdaters {
//	        fn(up) // 通知所有注册的回调（通常是父级负载均衡器）
//	    }
//	}
type StatusUpdater interface {
	RegisterStatusUpdater(fn func(up bool)) error
}

// target 表示一个健康检查目标（后端服务）。
//
// 它把逻辑名称和后端 URL 绑定在一起，在 healthyTargets/unhealthyTargets
// 两个 channel 之间流转。
//
// 字段说明：
//
//	name:     逻辑名称，用于状态通知（balancer.SetStatus 的 childName 参数）
//	          对于 LoadBalancer 类型，值等于 server.Address（如 "10.0.0.1:8080"）
//	targetUrl: 完整的探测 URL，用于发起 HTTP/gRPC 健康检查请求
//
// 示例：
//
//	t := target{
//	    name:      "server-01",
//	    targetUrl: &url.URL{Scheme: "http", Host: "10.0.0.1:8080"},
//	}
//	// 在 healthCheck 循环中：
//	//   shc.executeHealthCheck(ctx, config, t.targetUrl) → 探测 10.0.0.1:8080
//	//   shc.balancer.SetStatus(ctx, t.name, result)     → 通知 "server-01" 状态
type target struct {
	name      string
	targetUrl *url.URL
}

// ServiceHealthChecker 主动健康检查器。
//
// 通过定时器定期向所有后端发起探测请求（HTTP 或 gRPC），
// 根据响应结果更新后端状态。健康的后端以正常频率检查，
// 故障的后端以更快的频率检查（unhealthyInterval），以便及时发现恢复。
//
// # 核心设计：双 channel 模式
//
// 健康/故障后端分别放入不同的 channel，用不同频率的 goroutine 消费：
//
//	┌──────────────────┐       interval（正常频率）     ┌──────────────────────┐
//	│  healthyTargets   │ ────────────────────────────→ │ 健康检查协程（主线程）  │
//	│  (chan target)    │                               │                      │
//	└──────────────────┘                               └──────┬───────────────┘
//	       ↑                                                  │ 检测到故障
//	       │ 恢复后放回                                        ↓
//	       │                                          ┌──────────────────┐
//	┌──────┴─────────────┐      unhealthyInterval     │  unhealthyTargets │
//	│ 故障检查协程(goroutine)│ ←─────────────────────── │  (chan target)    │
//	│                      │    （更快的频率）           └──────────────────┘
//	└──────────────────────┘
type ServiceHealthChecker struct {
	ctx          context.Context     // 生命周期上下文，ctx 取消时健康检查停止
	log          *zap.Logger         // 日志记录器
	roundTripper http.RoundTripper   // HTTP 传输层，用于复用连接池等底层配置
	targets      map[string]*url.URL // 所有待检查的后端，key 为名称，value 为 URL

	balancer StatusSetter            // 状态通知接口，检查结果通过它通知负载均衡器
	info     *runtimecfg.ServiceInfo // 运行时服务信息，用于更新后端状态快照（info.UpdateServerStatus）

	config            *dynamic.ServerHealthCheck // 健康检查配置（路径、方法、超时等）
	interval          time.Duration              // 健康后端的检查间隔
	unhealthyInterval time.Duration              // 故障后端的检查间隔（通常比 interval 短，以便更快发现恢复）
	timeout           time.Duration              // 单次健康检查的超时时间

	client *http.Client // HTTP 客户端，用于发起探测请求

	healthyTargets   chan target // 健康后端队列：当前健康的后端放入此 channel，按 interval 频率取出检查
	unhealthyTargets chan target // 故障后端队列：当前故障的后端放入此 channel，按 unhealthyInterval 频率取出检查

	serviceName string // 服务名称，用于日志标识
}

// ============================================================================
// ServiceHealthChecker 的 Options 模式构造器
//
// 使用 functional options 模式，每个 Set* 函数返回一个 options.Option，
// 在 NewServiceHealthChecker 中依次应用到实例上。
//
// 典型用法：
//
//	shc := healthcheck.NewServiceHealthChecker(
//	    healthcheck.SetServiceName("backend@file"),
//	    healthcheck.SetLogger(logger),
//	    healthcheck.SetCtx(ctx),
//	    healthcheck.SetBalancer(lb),
//	    healthcheck.SetInfo(svcInfo),
//	    healthcheck.SetConfig(&dynamic.ServerHealthCheck{...}),
//	    healthcheck.SetTargets(targets),
//	    healthcheck.SetRoundTripper(transport),
//	)
//
// 所有 Set* 函数共享相同的类型签名：func(...any) → options.Option，
// 内部通过类型断言将 any 转为 *ServiceHealthChecker 并设置对应字段。
// ============================================================================

// SetServiceName 设置服务名称，用于日志中标识是哪个服务的健康检查。
//
// 示例：
//
//	healthcheck.SetServiceName("my-backend@file")
//	// 后续日志输出：Health check result  service=my-backend@file  target=server1  status=UP
//
// 参数 serviceName 最终赋值给 ServiceHealthChecker.serviceName 字段。
func SetServiceName(serviceName string) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.serviceName = serviceName
	}
}

// SetBalancer 设置状态通知接口。
//
// 健康检查探测到后端状态变化后，通过此接口通知负载均衡器。
// 任何实现了 StatusSetter 接口的类型都可以传入。
//
// 示例：
//
//	lb := tcp.NewWRRLoadBalancer(logger, true) // 支持健康检查
//	healthcheck.SetBalancer(lb)                // lb 实现了 StatusSetter
//
// 注意：balancer 不能为 nil，否则在 healthCheck 中调用 SetStatus 时会 panic。
func SetBalancer(balancer StatusSetter) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.balancer = balancer
	}
}

// SetInfo 设置运行时服务信息。
//
// ServiceInfo 维护了服务级别的运行时状态，包括每个后端的健康状态快照。
// 健康检查结果会通过 info.UpdateServerStatus() 同步更新，供 API 查询使用。
//
// 示例：
//
//	svcInfo := conf.GetServiceInfo("my-backend@file")
//	healthcheck.SetInfo(svcInfo)
//	// 后续查询 API：GET /api/services/my-backend@file → 返回各后端状态快照
func SetInfo(info *runtimecfg.ServiceInfo) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.info = info
	}
}

// SetConfig 设置健康检查配置（路径、方法、超时等）。
//
// ServerHealthCheck 结构体包含以下可配置字段：
//
//	Path:              健康检查路径（如 "/health", "/ping"），必填
//	Interval:          健康后端检查间隔（纳秒），未设置时使用默认值
//	UnhealthyInterval: 故障后端检查间隔（纳秒），未设置时与 Interval 相同
//	Timeout:           单次检查超时（纳秒），未设置时使用默认值
//	Mode:              "grpc" 或不填（默认 HTTP）
//	Method:            HTTP 方法（如 GET, HEAD），默认 GET
//	Scheme:            覆盖后端的 scheme（如强制 https 检查 http 注册的后端）
//	Port:              覆盖后端的 port（如后端在 8080 但 health check 走 9090）
//	Hostname:          HTTP Host 头（虚拟主机场景）
//	Headers:           自定义请求头（如认证 token）
//	Status:            期望的状态码（0=使用默认规则 2xx/3xx），非 0=严格匹配
//	FollowRedirects:   是否跟随重定向
//
// 示例：
//
//	healthcheck.SetConfig(&dynamic.ServerHealthCheck{
//	    Path:     "/healthz",
//	    Interval: 10_000_000_000,  // 10s, 纳秒
//	    Timeout:  3_000_000_000,   // 3s
//	    Status:   200,             // 只有 HTTP 200 才算健康
//	    Mode:     "",              // 空字符串 = HTTP 模式
//	})
func SetConfig(config *dynamic.ServerHealthCheck) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.config = config
	}
}

// SetRoundTripper 设置 HTTP 传输层。
//
// RoundTripper 控制底层网络行为：连接池大小、TLS 配置、超时等。
// 通常传入 http.DefaultTransport 或自定义的 transport。
//
// 示例：
//
//	// 使用默认传输层
//	healthcheck.SetRoundTripper(http.DefaultTransport)
//
//	// 使用自定义传输层（跳过 TLS 验证）
//	customTransport := &http.Transport{
//	    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
//	}
//	healthcheck.SetRoundTripper(customTransport)
func SetRoundTripper(rp http.RoundTripper) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.roundTripper = rp
	}
}

// SetLogger 设置日志记录器。
//
// 所有健康检查日志（启动、超时、状态变化等）都会通过此 logger 输出。
//
// 示例：
//
//	logger, _ := zap.NewProduction()
//	healthcheck.SetLogger(logger)
func SetLogger(log *zap.Logger) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.log = log
	}
}

// SetCtx 设置生命周期上下文，ctx 取消时所有健康检查停止。
//
// context 的 Done() channel 被关闭时，healthCheck 的主循环会退出，
// 每个 ticker 周期、每个 target 检查前都会检查 ctx.Done()。
//
// 示例：
//
//	ctx, cancel := context.WithCancel(context.Background())
//	healthcheck.SetCtx(ctx)
//	// ... 启动健康检查 ...
//	// 服务关闭时：
//	cancel() // 优雅停止所有健康检查循环
func SetCtx(ctx context.Context) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.ctx = ctx
	}
}

// SetTargets 设置所有待检查的后端。
//
// key 为后端逻辑名称（用于状态通知），value 为后端 URL（用于发起探测）。
//
// 示例：
//
//	healthcheck.SetTargets(map[string]*url.URL{
//	    "server-1": {Scheme: "http", Host: "10.0.0.1:8080"},
//	    "server-2": {Scheme: "http", Host: "10.0.0.2:8080"},
//	    "server-3": {Scheme: "http", Host: "10.0.0.3:8080"},
//	})
//	// 初始状态：所有 3 个后端都放入 healthyTargets channel
//	// 健康检查循环会对每个后端按 interval 频率发起探测
func SetTargets(targets map[string]*url.URL) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.targets = targets
	}
}

// NewServiceHealthChecker 创建主动健康检查器。
//
// 使用 functional options 模式注入所有依赖，然后校验和补齐配置：
//  1. 检查间隔 (interval): 未配置或 <= 0 时使用默认值 DefaultHealthCheckInterval
//  2. 故障检查间隔 (unhealthyInterval): 未配置时与 interval 相同，即一视同仁；
//     配置了且 > 0 则使用配置值，<= 0 则回退到默认值
//  3. 超时时间 (timeout): 未配置或 <= 0 时使用默认值 DefaultHealthCheckTimeout
//  4. 重定向策略: 配置了 FollowRedirects=false 时，禁用 HTTP 客户端自动跟随重定向，
//     因为重定向后的目标健康不代表原始目标健康
//  5. 初始化两个 channel: 所有后端初始都是健康的，放入 healthyTargets；
//     unhealthyTargets 初始为空，等待后续有后端变故障时才会被填入
//
// 返回配置完毕但尚未启动的健康检查器，需要调用 Launch() 启动。
//
// 使用示例：
//
//	shc := NewServiceHealthChecker(
//	    SetServiceName("my-backend@file"),
//	    SetLogger(logger),
//	    SetCtx(ctx),
//	    SetBalancer(lb),
//	    SetInfo(svcInfo),
//	    SetConfig(&dynamic.ServerHealthCheck{
//	        Path:     "/health",
//	        Interval: 10_000_000_000,
//	        Timeout:  3_000_000_000,
//	        Status:   200,
//	    }),
//	    SetRoundTripper(http.DefaultTransport),
//	    SetTargets(map[string]*url.URL{
//	        "s1": {Scheme: "http", Host: "10.0.0.1:8080"},
//	    }),
//	)
//	// 此时健康检查已配置但未启动
//	go shc.Launch(ctx) // 在 goroutine 中异步启动
func NewServiceHealthChecker(opts ...options.Option) *ServiceHealthChecker {
	shc := &ServiceHealthChecker{}
	// 依次应用所有 options，每个 option 设置一个字段
	for _, opt := range opts {
		opt(shc)
	}

	// ── Interval 校验 ──
	// 健康后端的检查间隔，控制多久对状态为 UP 的后端发起一次探测。
	//
	// time.Duration(shc.config.Interval)：
	//   配置中的 Interval 字段如果为 nil 或未设置，Go 的 json 反序列化会给它赋零值 0，
	//   time.Duration(0) = 0ns。
	//
	// if interval <= 0 为 true 的情况：
	//   - shc.config.Interval == nil 或未在配置文件中设置 → interval = time.Duration(0) = 0ns → 进入此分支
	//   - 用户显式设置了负值或 0（如 Interval: 0 或 Interval: -1）→ 无效值，使用默认
	//   → 打印警告，回退到 DefaultHealthCheckInterval
	//
	// if interval <= 0 为 false：
	//   用户配置了有效的正值（如 10s）→ 使用配置值
	interval := time.Duration(shc.config.Interval)
	if interval <= 0 {
		shc.log.Warn("Health check interval smaller than zero, default value will be used instead.")
		interval = time.Duration(dynamic.DefaultHealthCheckInterval)
	}

	// ── UnhealthyInterval 校验 ──
	// 故障后端的检查间隔，通常设置得比 interval 短，以便更快发现后端恢复。
	// 如果未配置 UnhealthyInterval（为 nil），则与 interval 相同，健康/故障使用统一频率。
	//
	// if shc.config.UnhealthyInterval != nil 为 true 的分支：
	//   用户显式配置了故障检查间隔 → 进一步校验其值是否合法
	//
	//   ── 嵌套 if unhealthyInterval <= 0 为 true：
	//       用户配置了 <= 0（如 0 或 -1）→ 打印警告，回退到默认值
	//
	//   ── 嵌套 if unhealthyInterval <= 0 为 false：
	//       配置值合法（正值）→ 使用配置值
	//
	// if shc.config.UnhealthyInterval != nil 为 false（即 else 分支）：
	//   用户没有配置 UnhealthyInterval → 与健康检查间隔保持一致
	//   这意味着健康/故障后端使用相同的检查频率（无差异化检测）
	//
	// 举例：
	//   interval=10s, UnhealthyInterval=nil      → 健康=10s, 故障=10s（统一频率）
	//   interval=10s, UnhealthyInterval=3s       → 健康=10s, 故障=3s（故障更快检测）
	//   interval=10s, UnhealthyInterval=0        → 健康=10s, 故障=DefaultHealthCheckInterval（回退默认）
	var unhealthyInterval time.Duration
	if shc.config.UnhealthyInterval != nil {
		unhealthyInterval = time.Duration(*shc.config.UnhealthyInterval)
		if unhealthyInterval <= 0 {
			shc.log.Warn("Unhealthy health check interval smaller than zero, default value will be used instead.")
			unhealthyInterval = time.Duration(dynamic.DefaultHealthCheckInterval)
		}
	} else {
		unhealthyInterval = interval
	}

	// ── Timeout 校验 ──
	// 单次健康检查请求的超时时间，防止后端不响应导致协程泄漏。
	// 每个健康检查都用 context.WithDeadline 设置了截止时间。
	//
	// if timeout <= 0 为 true：
	//   - 未配置 Timeout（json 零值 0）
	//   - 用户配置了 <=0 的无效值
	//   → 打印警告，回退到 DefaultHealthCheckTimeout
	//
	// if timeout <= 0 为 false：
	//   配置值合法 → 使用配置值
	//
	// 最佳实践：timeout 应该小于 interval，否则可能在上一次检查完成前就触发下一次检查。
	// 例如：interval=10s, timeout=3s → 每次有 7s 空闲时间，合理。
	//       interval=10s, timeout=15s → 可能并发执行两次检查，不合理。
	timeout := time.Duration(shc.config.Timeout)
	if timeout <= 0 {
		shc.log.Warn("Health check timeout smaller than zero, default value will be used instead.")
		timeout = time.Duration(dynamic.DefaultHealthCheckTimeout)
	}

	// ── HTTP 客户端 ──
	// 使用独立的 http.Client，通过 roundTripper 复用连接池配置。
	// client.Timeout 不设置，因为每次请求用 context.WithDeadline 单独控制超时。
	client := &http.Client{
		Transport: shc.roundTripper,
	}

	// ── 重定向策略 ──
	// Go 的 http.Client 默认自动跟随重定向（最多 10 次）。
	// 但健康检查要探测的是目标本身的状态，不是重定向后的目标。
	//
	// 示例场景：后端 A 返回 302 Location: http://backend-B/health
	//   - 如果跟随重定向：B 返回 200 → 误判 A 是健康的（而 A 可能已经不可用）
	//   - 如果不跟随重定向：收到 302 → 根据 Status 配置判断是否健康
	//
	// if shc.config.FollowRedirects != nil && !*shc.config.FollowRedirects 为 true：
	//   用户显式设置了 FollowRedirects=false → 禁用跟随重定向
	//   通过 CheckRedirect 回调返回 http.ErrUseLastResponse（一种 Go 惯用法），
	//   告诉客户端把重定向响应本身当作最终结果返回，不继续跟随。
	//
	// 需要同时满足两个条件：
	//   1. FollowRedirects != nil：用户配置了该字段（区分"未设置"和"设置为false"）
	//   2. !*shc.config.FollowRedirects：用户将其设置为 false
	//
	// if 条件为 false（任一条件不满足）：
	//   - FollowRedirects == nil：未配置 → 使用 Go 默认行为（跟随重定向）
	//   - FollowRedirects == true：显式允许 → 使用 Go 默认行为（跟随重定向）
	if shc.config.FollowRedirects != nil && !*shc.config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// ── 初始化 channel ──
	// 创建带缓冲的 channel，缓冲大小等于后端数量，避免在初始填充时发送阻塞。
	//
	// 初始状态：所有后端都是健康的 → 全部放入 healthyTargets channel。
	// unhealthyTargets 初始为空，等待后续有后端变故障时才会被填入。
	//
	// 举例：
	//   初始状态：
	//     healthyTargets   ← {server1, server2, server3}（3 个后端全部健康）
	//     unhealthyTargets ← {}（空）
	//
	//   运行一段时间后 server2 故障：
	//     healthyTargets   ← {server1, server3}（server2 被移出）
	//     unhealthyTargets ← {server2}（server2 被移入，开始更频繁检查）
	//
	// 为什么用带缓冲的 channel 而非无缓冲的？
	//   for 循环中初始填充 target 时，如果 channel 无缓冲，每次 send 会阻塞等待接收方，
	//   但接收方（healthCheck goroutine）还没有启动 → 死锁。
	//   带缓冲的 channel 允许发送方在缓冲未满时不阻塞地发送。
	healthyTargets := make(chan target, len(shc.targets))
	unhealthyTargets := make(chan target, len(shc.targets))
	for name, targetUrl := range shc.targets {
		healthyTargets <- target{name: name, targetUrl: targetUrl}
	}

	// 将校验后的值写回结构体
	shc.interval = interval
	shc.unhealthyInterval = unhealthyInterval
	shc.timeout = timeout
	shc.client = client
	shc.healthyTargets = healthyTargets
	shc.unhealthyTargets = unhealthyTargets

	return shc
}

// Launch 启动主动健康检查。
//
// 采用双 goroutine 模式：
//   - 故障检查协程（异步启动）：从 unhealthyTargets channel 消费，按 unhealthyInterval 频率检查故障后端
//   - 健康检查协程（同步启动）：从 healthyTargets channel 消费，按 interval 频率检查健康后端
//
// 健康检查协程不是异步的（没有加 go），这意味着 Launch 会阻塞当前协程。
// 通常调用方需要自己 `go shc.Launch(ctx)` 来异步启动。
//
// 两个协程通过 channel 互相投喂：健康检查发现后端故障时，该后端被放入 unhealthyTargets；
// 故障检查发现后端恢复时，该后端被放回 healthyTargets。形成闭环。
//
// 为什么健康检查在主协程中同步运行？
//
//	这是一种简化设计：避免过多 goroutine。调用方多加一个 `go` 即可异步。
//	如果两个都异步启动，Launch 立即返回，调用方难以判断何时启动完成。
//
// 使用示例：
//
//	shc := NewServiceHealthChecker(...)
//
//	// 方式一：在独立 goroutine 中启动（推荐）
//	go shc.Launch(ctx)
//
//	// 方式二：启动多个健康检查器（如多个服务）
//	for _, svc := range services {
//	    go svc.Launch(ctx)
//	}
//
//	// 停止所有健康检查：
//	cancel() // 触发 ctx.Done()，所有 goroutine 优雅退出
func (shc *ServiceHealthChecker) Launch(ctx context.Context) {
	shc.log.Info("Launching unhealthy checker", zap.String("service", shc.serviceName))
	// 故障检查协程异步启动，不阻塞 Launch 主流程
	go shc.healthCheck(ctx, shc.unhealthyTargets, shc.unhealthyInterval)

	shc.log.Info("Launching healthy checker", zap.String("service", shc.serviceName))
	// 健康检查在主协程中阻塞运行，直到 ctx 取消
	// 如果这里也加 go，Launch 会立即返回，调用方不知道何时启动完成
	shc.healthCheck(ctx, shc.healthyTargets, shc.interval)
}

// healthCheck 是健康检查的核心循环。
//
// 工作原理：
//  1. 创建 ticker，按 interval 频率触发检查
//  2. 每次 tick 触发时，从 channel 中批量取出当前所有待检查的 target
//     （非阻塞地一次性清空 channel，用完为止，不等下一个，减少单次检查时间）
//  3. 对每个 target 发起探测请求（HTTP 或 gRPC）
//  4. 根据探测结果：
//     - 成功 → 将 target 放回 healthyTargets，更新状态为 UP
//     - 失败 → 将 target 放入 unhealthyTargets，更新状态为 DOWN
//  5. 如果 ctx 被取消（配置刷新），立即退出
//
// 关键设计：target 在两个 channel（healthyTargets/unhealthyTargets）之间流转，
// 而不是直接用 map 维护状态。这样设计的好处是：
//   - 避免对 map 加锁的竞争
//   - target 的数量动态变化时不会有遗漏
//   - 健康和不健康用不同频率检查，天然隔离
//
// 参数说明：
//
//	ctx:      从 Launch 传入，用于控制整个循环的生命周期
//	targets:  消费的 channel（healthyTargets 或 unhealthyTargets）
//	interval: 检查间隔（interval 或 unhealthyInterval）
func (shc *ServiceHealthChecker) healthCheck(ctx context.Context, targets chan target, interval time.Duration) {
	// 创建定时器，每隔 interval 时间触发一次检查
	// time.NewTicker 底层使用单调时钟，不受系统时间调整（NTP）影响
	ticker := time.NewTicker(interval)
	defer ticker.Stop() // 函数退出时释放 ticker 资源，防止 goroutine 泄漏

	for {
		// ── 最外层 select：两种退出路径 ──
		// select 是 Go 的非确定性多路复用：
		//   - 如果 ctx.Done() 和 ticker.C 同时就绪 → 随机选择其一
		//   - 如果只有 ctx.Done() 就绪 → 退出循环
		//   - 如果只有 ticker.C 就绪 → 执行健康检查
		select {
		case <-ctx.Done():
			// ── 退出路径 1：context 取消 ──
			// ctx.Done() 返回的 channel 被关闭时，此 case 立即可读。
			//
			// ctx 取消的常见场景：
			//   - 服务关停（SIGTERM, SIGINT）
			//   - 配置刷新（旧的配置 context 被 cancel）
			//   - 手动调用 cancel()
			//
			// select 读到已关闭 channel 时零值立即返回（非阻塞），
			// 读取到的值总是零值（struct{}{}），可以直接 return。
			return

		case <-ticker.C:
			// ── 退出路径 2：ticker 触发 ──
			// ticker.C 在每次 interval 到达时发送当前时间（time.Time）。
			// 第一次触发在 interval 之后（不是立即触发）。
			//
			// 举例：interval=10s
			//   t=0:    ticker 创建
			//   t=10s:  ticker.C 第一次触发（第一批健康检查）
			//   t=20s:  ticker.C 第二次触发
			//   ...

			// ── 第一层：批量收集当前所有待检查的 target ──
			// 使用 for + select + default 模式，非阻塞地一次性取出 channel 中
			// 所有立即可用的 target，攒够了就统一处理，不等空 channel 阻塞。
			//
			// 为什么需要批量收集？效率考虑：
			//   如果每次 ticker 只取一个 target，有 N 个后端需要 N 个 interval
			//   才能全量检查一轮 → 太慢。批量收集后一次检查所有 target。
			//
			// 为什么用 hasMoreTargets 布尔变量而不是 break？
			//   因为 select 中的 break 只能跳出 select 本身，不能跳出外层 for 循环。
			//   如果不使用标签 break 或 bool 变量，break 只会退出 select，
			//   然后回到 for 开头继续 select → default → break → 死循环。
			//
			// 等价逻辑（用标签 break）：
			//   loop:
			//     for {
			//       select {
			//       case t := <-targets: ...
			//       default: break loop
			//       }
			//     }
			var targetsToCheck []target
			hasMoreTargets := true
			for hasMoreTargets {
				select {
				case <-ctx.Done():
					// 批量收集过程中 ctx 被取消 → 立即退出
					return
				case t := <-targets:
					// channel 中有数据，收集到切片中
					// 注意：append 可能会分配新的底层数组（扩容），
					// 但这里只有 len(shc.targets) 个 target，扩容次数有限
					targetsToCheck = append(targetsToCheck, t)
				default:
					// channel 中没有立即可用的数据了，停止收集
					hasMoreTargets = false
				}
			}

			// ── 第二层：逐个对收集到的 target 执行健康检查 ──
			// 遍历顺序就是 ticks 周期中 channel 的消费顺序（FIFO），
			// 不存在优先级概念 — 所有 target 一视同仁。
			for _, t := range targetsToCheck {
				// 在每次健康检查前，非阻塞地检查 ctx 是否已取消。
				//
				// 为什么需要空 default？
				//   如果去掉 default，select 只有一个 case <-ctx.Done()，
				//   当 ctx 未取消时，select 会永久阻塞在这个 case 上，
				//   导致后面的 executeHealthCheck 永远执行不到 → 整个循环卡死。
				//   空 default 让 select "不阻塞"：ctx 取消了就 return，没取消就继续往下走。
				//
				// 为什么不在 for range 外层统一检查一次？
				//   如果 targetsToCheck 数量很多（如 100 个后端），每个检查耗时 1s，
				//   外层检查一次后需要 100s 才能走完，如果中间 ctx 被取消了，
				//   还需要等剩余的检查完成才能退出 → 响应太慢。
				//   每个 target 检查前都检查一次，能在 1s 内响应取消。
				select {
				case <-ctx.Done():
					return
				default:
					// ctx 尚未取消，跳过，继续执行健康检查
				}

				up := true
				// 执行实际的健康检查探测（HTTP 或 gRPC）
				// executeHealthCheck 内部用 context.WithDeadline 控制单次超时
				if err := shc.executeHealthCheck(ctx, shc.config, t.targetUrl); err != nil {
					// ── 判断错误类型 ──
					// errors.Is 沿错误链向上查找，检查是否包含 context.Canceled。
					// executeHealthCheck 的 gRPC 分支中，codes.Canceled 被显式转为 context.Canceled。
					//
					// if errors.Is(err, context.Canceled) 为 true：
					//   ctx 被外部取消（配置刷新、服务关停等），不是真正的健康检查失败，
					//   不需要记录错误日志，直接退出循环
					//
					// if errors.Is(err, context.Canceled) 为 false：
					//   真正的探测失败（超时、连接被拒、状态码不匹配等），
					//   记录错误日志，标记 up=false
					if errors.Is(err, context.Canceled) {
						return
					}

					shc.log.Error("Health check failed", zap.String("service", shc.serviceName), zap.String("target", t.name), zap.Error(err))
					up = false
				}

				// ── 第1步：通知负载均衡器状态变化 ──
				// balancer 就是 WRRLoadBalancer，SetStatus 内部加锁更新状态 map
				shc.balancer.SetStatus(ctx, t.name, up)

				// ── 第2步：根据结果将 target 路由到正确的 channel ──
				// 这是双 channel 模式的核心：target 在两个 channel 之间流转。
				//
				// 发送操作可能阻塞（channel 满时），但已经创建了足够大的缓冲
				// （等于 len(targets)），正常情况下不会阻塞。
				// 极端情况：如果某种 bug 导致 channel 中的 target 数量超过了
				// 缓冲大小（如某个 target 被多次放入），send 会阻塞。
				// 所以 channel 的设计是"取出再放回"，同一时间 channel 中
				// 的 target 数量不会超过缓冲大小。
				var statusStr string
				if up {
					// 后端健康 → 放回 healthyTargets（下次按 interval 频率检查）
					// 使用 healthyTargets 字段而非参数 targets，确保放回正确的 channel
					statusStr = runtimecfg.StatusUp
					shc.healthyTargets <- t
				} else {
					// 后端故障 → 放入 unhealthyTargets（下次按 unhealthyInterval 频率检查）
					// unhealthyInterval 通常更短，故障后端会被更频繁地检查
					statusStr = runtimecfg.StatusDown
					shc.unhealthyTargets <- t
				}

				// ── 第3步：更新运行时状态快照 ──
				// info.UpdateServerStatus 更新内存中的状态快照，
				// 供 API 查询使用（GET /api/services/{name}）。
				// key 为目标 URL 字符串（如 "http://10.0.0.1:8080"）
				shc.info.UpdateServerStatus(t.targetUrl.String(), statusStr)

				shc.log.Info("Health check result", zap.String("service", shc.serviceName), zap.String("target", t.name), zap.String("status", statusStr))
			}
		}
	}
}

const modeGRPC = "grpc" // gRPC 模式标识，与 HTTP 默认模式区分

// executeHealthCheck 执行单次健康检查，根据配置的 Mode 字段分发到 HTTP 或 gRPC 检查。
//
// 每次检查前创建一个带 deadline 的子 context：
//   - deadline = 当前时间 + timeout
//   - deadline 到达时，进行中的 HTTP/gRPC 请求会被自动取消，避免协程泄漏
//   - defer cancel() 确保子 context 在函数返回时被释放，防止 context 泄漏
//
// 路由逻辑（switch config.Mode）：
//
//	if config.Mode == "grpc" → 调用 gRPC 健康检查
//	else                    → 调用 HTTP 健康检查（默认模式，包括空字符串、未设置、"http"等）
func (shc *ServiceHealthChecker) executeHealthCheck(ctx context.Context, config *dynamic.ServerHealthCheck, target *url.URL) error {
	// context.WithDeadline 创建带绝对截止时间的子 context：
	//   - time.Now().Add(shc.timeout) = 截止时间点
	//   - 到达截止时间后，子 context 的 Done() channel 被关闭
	//   - 子 context 的 Done() 关闭会逐层传递到 HTTP/gRPC 请求中
	//   - 不使用 context.WithTimeout(shc.timeout) 是因为底层也是用 WithDeadline 实现的
	//
	// 注意：这里使用的是传入的 ctx（父 context），它会继承父 context 的取消信号。
	// 如果父 ctx 先于 deadline 被取消，子 context 也会立即被取消。
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(shc.timeout))
	defer cancel() // 函数返回时释放 context 资源，防止 goroutine 泄漏

	// switch config.Mode 的类型判断：
	//   - case modeGRPC（即 "grpc"）→ 走 gRPC 路径
	//   - default → 走 HTTP 路径（包括 Mode=""、Mode="http"、Mode 未设置等所有非 "grpc" 的情况）
	//
	// 为什么用 default 而不是显式 case "http"？
	//   这是"允许性设计"：未来如果新增模式（如 "tcp"），在添加新 case 之前，
	//   会先 fallback 到 HTTP 而不是报错，保证向前兼容。
	switch config.Mode {
	case modeGRPC:
		// gRPC 健康检查：使用 gRPC Health Checking Protocol v1
		// 协议定义：https://github.com/grpc/grpc/blob/master/doc/health-checking.md
		if err := shc.checkHealthGRPC(ctx, target); err != nil {
			return fmt.Errorf("grpc health check failed: %w", err)
		}
	default:
		// 默认 HTTP 健康检查：发起 HTTP 请求，检查响应状态码
		if err := shc.checkHealthHTTP(ctx, target); err != nil {
			return fmt.Errorf("http health check failed: %w", err)
		}
	}

	return nil
}

// checkHealthHTTP 执行 HTTP 健康检查。
//
// 判断逻辑（两阶段判断，第一阶段优先）：
//  1. config.Status == 0（未配置期望状态码）：
//     使用默认规则：2xx 和 3xx 视为健康，4xx 及以上视为故障
//  2. config.Status != 0（配置了期望状态码）：
//     严格匹配：只有响应状态码 == 配置值才视为健康
//
// 注意：defer resp.Body.Close() 确保响应体被关闭，否则会导致连接泄漏。
//
// 使用示例：
//
//	// 默认规则（Status=0）：200-399 均视为健康
//	config := &dynamic.ServerHealthCheck{Path: "/health", Status: 0}
//	// 后端返回 200 → 健康
//	// 后端返回 302 → 健康（3xx 是重定向，服务本身在运行）
//	// 后端返回 404 → 故障
//
//	// 严格匹配（Status=200）：
//	config := &dynamic.ServerHealthCheck{Path: "/health", Status: 200}
//	// 后端返回 200 → 健康
//	// 后端返回 301 → 故障（不符合 200）
func (shc *ServiceHealthChecker) checkHealthHTTP(ctx context.Context, target *url.URL) error {
	// 构造健康检查 HTTP 请求（路径、端口、scheme、header 等由 newRequest 处理）
	req, err := shc.newRequest(ctx, target)
	if err != nil {
		// 请求构造失败（如 path 为绝对 URL 被安全校验拦截）→ 视为检查失败
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 发起 HTTP 请求。shc.client 可能已配置了不跟随重定向的策略（CheckRedirect）。
	resp, err := shc.client.Do(req)
	if err != nil {
		// 网络层错误：连接被拒、DNS 解析失败、TLS 握手失败、超时等
		// 这类错误直接返回，不再进行状态码判断（因为根本没有状态码）
		return fmt.Errorf("failed to execute request: %w", err)
	}
	// defer 注册在 err 判断之后，避免 resp 为 nil 时调用 resp.Body.Close() panic。
	// 必须关闭响应体：即使不读取 body，也必须关闭以释放 TCP 连接回连接池。
	defer resp.Body.Close()

	// ── 第一阶段判断：Status 为 0（默认规则） ──
	// shc.config.Status == 0：用户没有显式设置期望状态码
	//   → 使用默认规则：200 ≤ statusCode < 400 即为健康
	//
	// http.StatusOK = 200, http.StatusBadRequest = 400
	// resp.StatusCode >= 400 → 4xx 客户端错误或 5xx 服务端错误 → 故障
	// resp.StatusCode < 200  → 1xx 信息性响应（如 100 Continue）→ 不常见，视为故障
	//
	// 为什么 3xx 也算健康？
	//   3xx 是重定向，说明目标服务仍在正常运行，只是指示客户端去另一个地址。
	//   如果禁止跟随重定向（FollowRedirects=false），客户端会直接拿到 3xx 响应。
	//   例如：后端 /health → 302 → /login，302 本身不代表服务故障。
	//
	// 条件逻辑：Status==0 且 (statusCode<200 或 statusCode>=400) → 故障
	//          Status==0 且 200<=statusCode<=399 → 不进入此分支 → 健康
	//
	// 为什么用两个 && 分开而非一个范围判断？
	//   resp.StatusCode < http.StatusOK（<200）或 resp.StatusCode >= http.StatusBadRequest（>=400）
	//   等价于：! (200 <= StatusCode <= 399)
	//   这样写更清晰地表达了"非成功范围"的含义。
	if shc.config.Status == 0 && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest) {
		return fmt.Errorf("received error status code: %v", resp.StatusCode)
	}

	// ── 第二阶段判断：Status 非 0（精确匹配） ──
	// shc.config.Status != 0：用户配置了期望的特定状态码（如 200、204）
	//   → 只有响应状态码严格等于配置值才算健康
	//
	// if shc.config.Status != 0 && resp.StatusCode != shc.config.Status 为 true：
	//   用户配置了期望值但实际不匹配 → 故障
	//
	// 如果 Status==0（第一阶段）且 statusCode 在 [200,399] 之间，
	//   第一阶段条件不成立，第二阶段 Status!=0 也不成立 → 两个 if 都不进入 → return nil（健康）
	if shc.config.Status != 0 && resp.StatusCode != shc.config.Status {
		return fmt.Errorf("received status code: %v, expected: %v", resp.StatusCode, shc.config.Status)
	}

	return nil
}

// newRequest 构造健康检查 HTTP 请求。
//
// URL 组装优先级（由低到高，后者覆盖前者）：
//  1. target 的 host + path → 拼出基础 URL
//  2. config.Scheme → 覆盖 scheme（如强制用 https 替代 http）
//  3. config.Port → 覆盖端口（如后端注册了 8080 但健康检查走 9090）
//  4. config.Hostname → 设置 HTTP Host 头（用于虚拟主机场景）
//  5. config.Headers → 添加自定义请求头（如认证 token）
//
// Path 安全校验：path 必须是相对 URL（不含 host 和 scheme），
// 防止恶意配置注入绝对路径绕过检查目标。
func (shc *ServiceHealthChecker) newRequest(ctx context.Context, target *url.URL) (*http.Request, error) {
	// ── Path 解析与安全校验 ──
	// 解析配置中的健康检查路径（如 "/health"）
	pathURL, err := url.Parse(shc.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	// 安全校验：健康检查路径必须是相对路径。
	// 如果包含了 scheme 或 host（如 "http://evil.com/health"），
	// 攻击者可以通过配置注入，让健康检查探测到外部地址。
	if pathURL.Host != "" || pathURL.Scheme != "" {
		return nil, fmt.Errorf("health check path must be a relative URL, got: %q", shc.config.Path)
	}

	// ── URL 组装 ──
	// 以 target URL 为基准，使用 url.URL.Parse() 拼接相对路径
	// 例如：target="http://10.0.0.1:8080" + path="/health" → "http://10.0.0.1:8080/health"
	u, err := target.Parse(shc.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	// 如果配置了 Scheme，覆盖 target 的 scheme
	// 场景：后端注册时用 http，但健康检查要求走 https
	if len(shc.config.Scheme) > 0 {
		u.Scheme = shc.config.Scheme
	}

	// 如果配置了 Port，覆盖 target 的端口
	// 场景：后端注册在 8080，但健康检查端点暴露在 9090
	// 使用 net.JoinHostPort 确保 IPv6 地址正确格式化（加上方括号）
	if shc.config.Port != 0 {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(shc.config.Port))
	}

	// ── 构造 HTTP 请求 ──
	req, err := http.NewRequestWithContext(ctx, shc.config.Method, u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 如果配置了 Hostname，设置 HTTP Host 头
	// 场景：虚拟主机环境下，需要指定特定的 Host 头才能路由到正确的后端
	if shc.config.Hostname != "" {
		req.Host = shc.config.Hostname
	}

	// 添加自定义请求头
	// 场景：后端要求认证 token 或特定 header 才响应健康检查
	for k, v := range shc.config.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// checkHealthGRPC 执行 gRPC 健康检查。
//
// 基于 gRPC Health Checking Protocol v1 协议（grpc_health_v1.Health）。
//
// 流程：
//  1. 确定目标地址（端口优先使用 config.Port，否则用 serverURL 的端口）
//  2. 根据 scheme 决定是否使用 insecure 连接（h2c 表示明文 HTTP/2）
//  3. 调用 gRPC Health.Check 方法
//  4. 根据响应状态码判断健康状态（SERVING=健康，其他=故障）
//
// 使用示例：
//
//	config := &dynamic.ServerHealthCheck{
//	    Mode:   "grpc",     // 必须设置为 "grpc"
//	    Port:   50051,      // gRPC 端口（可选，不设置则用 target 的端口）
//	    Scheme: "h2c",      // h2c=明文 HTTP/2，空=""=TLS
//	}
//	target := &url.URL{Host: "10.0.0.1:9090", Scheme: "http"}
//	// 实际连接地址：10.0.0.1:50051（config.Port 覆盖）
func (shc *ServiceHealthChecker) checkHealthGRPC(ctx context.Context, serverURL *url.URL) error {
	// ── 确定端口 ──
	// 优先使用健康检查配置中指定的 Port，未配置则使用 serverURL 自带的端口。
	//
	// 为什么需要独立的 Port 配置？
	//   gRPC 服务可能监听多个端口（如 9090 用于业务，50051 用于健康检查），
	//   但注册在负载均衡器中的地址是业务端口。独立配置 Port 可以让健康检查
	//   走专用端口，不干扰业务流量。
	//
	// serverURL.Port() 返回端口部分（如 "9090"），不含冒号。
	// 如果 URL 中没有端口，返回空字符串（scheme 默认端口不单独返回）。
	port := serverURL.Port()
	if shc.config.Port != 0 {
		// shc.config.Port != 0：用户配置了健康检查专用端口
		//   → 用 strconv.Itoa 将 int 转为 string，覆盖 serverURL 的端口
		port = strconv.Itoa(shc.config.Port)
	}
	// 如果 shc.config.Port == 0 且 serverURL.Port() 也为 ""（无端口）：
	//   port = "" → JoinHostPort 会返回 "hostname:"（不合法，后续 Dial 会失败）
	//   但实际使用中 target 通常由负载均衡器构建，必定包含端口。

	// 拼接地址：hostname:port，JoinHostPort 自动处理 IPv6 方括号
	// 示例：
	//   Hostname="10.0.0.1", port="50051" → "10.0.0.1:50051"
	//   Hostname="::1", port="50051"      → "[::1]:50051"
	serverAddr := net.JoinHostPort(serverURL.Hostname(), port)

	// ── 配置 gRPC 连接选项 ──
	// 对于 http、h2c（明文 HTTP/2）或空 scheme，使用 insecure 连接。
	// 否则默认使用 TLS（生产环境）。
	//
	// switch shc.config.Scheme 的值匹配：
	//
	// case "http", "h2c", "":
	//   - "http": 显式指定明文
	//   - "h2c":  HTTP/2 Cleartext（无 TLS 的 HTTP/2）
	//   - "":    未设置 scheme，默认走 insecure
	//   → 追加 grpc.WithTransportCredentials(insecure.NewCredentials())
	//
	// 其他值（"https", "grpcs" 等）或未命中任何 case：
	//   → opts 为空切片，使用 gRPC 默认行为：系统根 CA 池 + TLS 证书验证
	var opts []grpc.DialOption
	switch shc.config.Scheme {
	case "http", "h2c", "":
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// ── 创建 gRPC 连接 ──
	// grpc.DialContext 创建一个 ClientConn：
	//   - 默认使用 DNS 解析器（dns:///）
	//   - 连接是惰性的：DialContext 返回时不代表连接已建立
	//   - 实际的连接建立发生在第一次 RPC 调用时
	//   - ctx 的 deadline 控制的是"等待连接变为 Ready 状态"的时间
	//
	// 为什么不设置 grpc.WithBlock()？
	//   不设置 WithBlock 时，DialContext 会立即返回（即使后端不可达），
	//   连接状态会异步地变为 TRANSIENT_FAILURE。
	//   后续 Check() 调用时才会真正尝试建立连接并受 ctx 控制。
	conn, err := grpc.DialContext(ctx, serverAddr, opts...)
	if err != nil {
		// DialContext 在以下情况返回错误（即使不设置 WithBlock）：
		//   - ctx 在 DialContext 返回前就已取消（deadline 已过或手动 cancel）
		//   - 解析目标地址失败（如 address 格式错误）
		//
		// if errors.Is(err, context.DeadlineExceeded) 为 true：
		//   ctx 的 deadline 在连接建立前就到达了
		//   → 返回明确的超时错误信息，包含地址和超时时间
		//
		// if errors.Is(err, context.DeadlineExceeded) 为 false：
		//   其他 Dial 错误（地址解析失败等）
		//   → 返回一般错误信息
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("fail to connect to %s within %s: %w", serverAddr, shc.config.Timeout, err)
		}
		return fmt.Errorf("fail to connect to %s: %w", serverAddr, err)
	}
	defer func() { _ = conn.Close() }() // 确保连接被关闭，错误被忽略（最佳实践）

	// ── 调用 gRPC Health Check API ──
	// healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	//   - 发起 gRPC 一元调用（Unary RPC）
	//   - HealthCheckRequest 可以带 Service 字段指定要检查的服务名，这里为空表示检查整个 server
	//   - ctx 中的 deadline 控制整个 RPC 的超时（包括连接建立 + 等待响应）
	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		// ── 解析 gRPC 状态码 ──
		// status.FromError(err) 将 gRPC 错误转为 *status.Status：
		//   - ok=true:   err 是一个 gRPC 状态错误（包含具体的 Code）
		//   - ok=false:  err 是普通网络错误（如连接断开），非 gRPC 状态错误
		//
		// if stat, ok := status.FromError(err); ok 为 true：
		//   进入了 gRPC 状态码分支，用 switch 精确区分不同错误码
		//
		// if stat, ok := status.FromError(err); ok 为 false：
		//   err 是网络层错误（非 gRPC 应用层）→ 跳过 switch，作为一般错误返回
		if stat, ok := status.FromError(err); ok {
			switch stat.Code() {
			case codes.Unimplemented:
				// gRPC 服务没有实现 Health 协议（grpc.health.v1.Health/Check）
				// → 视为故障。大多数 gRPC 服务都应该实现此协议。
				//
				// 注意：不能将 Unimplemented 降级为"跳过检查"，因为健康检查器
				// 不知道服务是否还有其他方式提供健康状态。
				return fmt.Errorf("gRPC server does not implement the health protocol: %w", err)
			case codes.DeadlineExceeded:
				// 检查超时：ctx 的 deadline 在收到响应之前到达
				// 这不是因为后端故障，而是因为后端响应太慢或死锁。
				return fmt.Errorf("gRPC health check timeout: %w", err)
			case codes.Canceled:
				// Context 被取消（通常是外部触发的配置刷新或服务关闭）
				// 返回 context.Canceled，由上层 healthCheck 的 errors.Is 判断处理
				// 注意：这里返回的是原始的 context.Canceled，而非包装后的错误，
				// 这样才能被 errors.Is(err, context.Canceled) 正确识别
				return context.Canceled
			}
			// 其他 gRPC 状态码（Unavailable, Internal, ResourceExhausted 等）
			// → 不单独处理，落到下面的统一错误格式
		}

		// 非 gRPC 状态错误或未匹配的 gRPC 状态码 → 统一错误
		return fmt.Errorf("gRPC health check failed: %w", err)
	}

	// ── 判断健康状态 ──
	// resp.GetStatus() 返回 HealthCheckResponse_ServingStatus 枚举：
	//   - SERVING (0):       健康，服务正常处理请求
	//   - NOT_SERVING (1):   不健康，服务启动但拒绝处理请求
	//   - UNKNOWN (2):       未知，服务未实现健康检查等
	//   - SERVICE_UNKNOWN (3): 未知的服务名称（请求中指定了不存在的 service）
	//
	// if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING 为 true：
	//   任何非 SERVING 的状态都视为故障 → 返回错误
	//
	// if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING 为 false：
	//   状态为 SERVING → 健康，跳过此分支，return nil
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("received gRPC status code: %v", resp.GetStatus())
	}

	return nil
}

// ============================================================================
// PassiveServiceHealthChecker — 被动健康检查（熔断器模式）
// ============================================================================
//
// 与主动健康检查不同，被动检查不主动发送探测请求，而是通过 WrapHandler
// 包装真实的 HTTP handler，拦截每一次真实请求的响应。
//
// 工作原理：
//   1. 每次后端请求返回 5xx 或网络错误时，记录失败时间
//   2. 在滑动时间窗口 (failureWindow) 内，如果失败次数达到阈值 (maxFailedAttempts)，
//      将后端标记为不健康（SetStatus → false）
//   3. 启动一个 timer，在 failureWindow 时间后自动将后端恢复为健康
//      （除非该服务也配置了主动健康检查，此时恢复由主动检查负责）
//
// 使用 singleflight 确保并发场景下只有一个 goroutine 执行状态更新逻辑，
// 配合 sync.Map 防止对同一后端重复创建恢复 timer。

type PassiveServiceHealthChecker struct {
	serviceName string       // 服务名称，用于日志标识
	balancer    StatusSetter // 状态通知接口，熔断/恢复时通知负载均衡器

	maxFailedAttempts    int            // 滑动窗口内允许的最大失败次数（阈值），超过则触发熔断
	failureWindow        types.Duration // 失败计数和熔断恢复的滑动时间窗口长度
	hasActiveHealthCheck bool           // 是否同时配置了主动健康检查
	//   true:  被动检查只负责熔断（SetStatus→false），恢复由主动检查负责
	//   false: 被动检查同时负责熔断和恢复（创建 timer 自动恢复）

	failuresMu sync.RWMutex           // 保护 failures map 的读写锁
	failures   map[string][]time.Time // key=targetURL（如 "http://10.0.0.1:8080"），value=失败时间列表（按时间升序排列）

	timersGroup singleflight.Group // 防止并发场景下对同一 target 重复执行熔断逻辑
	// singleflight.Group.Do(key, fn): 同一 key 的并发调用中，只有一个 goroutine 执行 fn，
	// 其他 goroutine 等待并共享结果。key 为 targetURL，确保每个后端只熔断一次。

	timers sync.Map // key=targetURL, value=*time.Timer（恢复定时器）
	// sync.Map 允许多 goroutine 无锁读取（timers.Load），减少了锁竞争。
	// 仅在创建 timer 时写入（timers.Store），timer 到期时删除（timers.Delete）。
}

// ============================================================================
// PassiveServiceHealthChecker 的 Options 模式构造器
//
// 与主动健康检查类似的 functional options 模式。
//
// 典型用法：
//
//	phc := healthcheck.NewPassiveHealthChecker(
//	    healthcheck.SetPassiveServiceName("my-backend@file"),
//	    healthcheck.SetPassiveBalancer(lb),
//	    healthcheck.SetPassiveMaxFailedAttempts(5),
//	    healthcheck.SetPassiveFailureWindow(types.Duration(30 * time.Second)),
//	    healthcheck.SetPassiveHasActiveHealthCheck(true), // 与主动检查共存
//	)
// ============================================================================

// SetPassiveServiceName 设置服务名称。
//
// 示例：
//
//	healthcheck.SetPassiveServiceName("backend-svc@kubernetes")
func SetPassiveServiceName(serviceName string) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.serviceName = serviceName
	}
}

// SetPassiveBalancer 设置状态通知接口，熔断/恢复时通过此接口通知负载均衡器。
//
// 示例：
//
//	phc := NewPassiveHealthChecker(
//	    SetPassiveBalancer(lb), // lb 实现了 StatusSetter 接口
//	)
func SetPassiveBalancer(balancer StatusSetter) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.balancer = balancer
	}
}

// SetPassiveMaxFailedAttempts 设置熔断阈值：滑动窗口内最大失败次数。
//
// 当 sliding window 内的失败次数达到此值时，触发熔断（SetStatus→false）。
//
// 示例：
//
//	// 窗口=30s, 最大失败=3 → 30s 内有 3 个请求失败就熔断
//	healthcheck.SetPassiveMaxFailedAttempts(3)
//
// 建议值：3-10，太低容易误熔断（偶发故障），太高失去保护意义。
func SetPassiveMaxFailedAttempts(maxFailedAttempts int) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.maxFailedAttempts = maxFailedAttempts
	}
}

// SetPassiveFailureWindow 设置滑动时间窗口：失败计数和熔断恢复的时间范围。
//
// 同时用于：
//   - 失败计数窗口：只统计 window 内的失败次数，窗口外的自动过期
//   - 熔断恢复时间：触发熔断后，window 时间后自动恢复
//
// 示例：
//
//	// 窗口=10s：10s 内的失败次数达到阈值则熔断，10s 后自动恢复
//	healthcheck.SetPassiveFailureWindow(types.Duration(10 * time.Second))
func SetPassiveFailureWindow(failureWindow types.Duration) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.failureWindow = failureWindow
	}
}

// SetPassiveHasActiveHealthCheck 标记是否同时配置了主动健康检查。
//
// if hasActiveHealthCheck == true:
//
//	被动检查只负责熔断（SetStatus→false），不创建恢复 timer。
//	恢复由主动健康检查（定时探测）负责。当主动检查发现后端恢复后，
//	它会独立调用 SetStatus→true。
//
// if hasActiveHealthCheck == false:
//
//	被动检查同时负责熔断和恢复。触发熔断后，创建一个 timer，
//	在 failureWindow 时间后自动调用 SetStatus→true 恢复。
//
// 示例：
//
//	// 主动+被动共存模式（推荐）
//	healthcheck.SetPassiveHasActiveHealthCheck(true)
//
//	// 仅被动模式
//	healthcheck.SetPassiveHasActiveHealthCheck(false)
func SetPassiveHasActiveHealthCheck(hasActiveHealthCheck bool) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.hasActiveHealthCheck = hasActiveHealthCheck
	}
}

// NewPassiveHealthChecker 创建被动健康检查器。
//
// NewPassiveHealthChecker 只负责构造，不启动任何 goroutine。
// 熔断逻辑在每次请求中通过 WrapHandler 惰性触发。
//
// 使用示例：
//
//	phc := NewPassiveHealthChecker(
//	    SetPassiveServiceName("my-backend@file"),
//	    SetPassiveBalancer(lb),
//	    SetPassiveMaxFailedAttempts(3),
//	    SetPassiveFailureWindow(types.Duration(10 * time.Second)),
//	)
//	// 用 phc.WrapHandler 包装后端代理 handler：
//	// httpsrv.Handle("/", phc.WrapHandler(ctx, proxyHandler, "http://10.0.0.1:8080"))
func NewPassiveHealthChecker(opts ...options.Option) *PassiveServiceHealthChecker {
	shc := &PassiveServiceHealthChecker{}
	for _, opt := range opts {
		opt(shc)
	}
	return shc
}

// WrapHandler 包装 HTTP handler，拦截后端请求的响应来判断被动健康状态。
//
// 这是被动健康检查的核心方法。它返回一个新的 http.Handler，该 handler：
//
//  1. 使用 httptrace.ClientTrace 检测是否真的与后端建立了连接（WroteHeaders/WroteRequest 回调）
//     防止将客户端层面的错误（如路由匹配失败返回 404）误判为后端故障。
//
//  2. 使用 codeCatcher 捕获后端返回的 HTTP 状态码，用于判断请求是否成功。
//
//  3. 请求成功（backendCalled=true 且 statusCode < 500）：
//     清空该 target 的失败记录（backendCalled=true 表明确实与后端通信了）
//
//  4. 请求失败（backendCalled=false 或 statusCode >= 500）：
//     记录当前时间到 failures 列表中
//
//  5. 通过 healthy() 判断是否触发熔断：
//     - 如果失败次数 < maxFailedAttempts → 不做处理
//     - 如果失败次数 >= maxFailedAttempts → 触发熔断
//
//  6. 熔断处理（使用 singleflight 防并发）：
//     - 检查是否已有恢复 timer 在运行（p.timers.Load）→ 是则跳过
//     - 否则 SetStatus(targetURL, false) 标记后端为不健康
//     - 启动一个 timer，在 failureWindow 后自动恢复 SetStatus(targetURL, true)
//     - 如果 hasActiveHealthCheck=true，则不启动恢复 timer，恢复交给主动检查
//
// 参数说明：
//
//	ctx:       生命周期上下文，用于控制 timer goroutine 的生命周期
//	next:      实际的 HTTP handler（如反向代理）
//	targetURL: 后端地址，同时作为 failutes map 和 timers sync.Map 的 key
//
// 使用示例：
//
//	// 简单用法：
//	proxy := httputil.NewSingleHostReverseProxy(targetURL)
//	wrapped := phc.WrapHandler(ctx, proxy, "http://10.0.0.1:8080")
//	http.Handle("/", wrapped)
//
//	// 多后端各用各的 WrapHandler：
//	for _, backend := range backends {
//	    proxy := httputil.NewSingleHostReverseProxy(backend.url)
//	    http.Handle(backend.path, phc.WrapHandler(ctx, proxy, backend.url.String()))
//	}
func (p *PassiveServiceHealthChecker) WrapHandler(ctx context.Context, next http.Handler, targetURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ── 第1步：安装 ClientTrace 检测是否真的与后端通信 ──
		// httptrace.ClientTrace 是 Go 标准库提供的 HTTP 客户端事件追踪机制。
		// 它会在各种网络事件发生时触发回调（DNS 解析、TCP 握手、写请求等）。
		//
		// 这里监听两个写入事件：
		//   - WroteHeaders: 请求头已写入 TCP 连接 → 说明连接已建立
		//   - WroteRequest:  整个请求体已写入 TCP 连接 → 说明请求已完整发送
		//
		// backendCalled=true 意味着请求确实发送到了后端，
		// 而非被中间件（如路由不匹配）直接返回错误。
		// 这样可以区分"后端故障"和"没有后端可用"两种情况。
		//
		// 为什么两个事件都设置 backendCalled=true？
		//   WroteHeaders 和 WroteRequest 可能不全触发：
		//     - 无 body 的 GET 请求：只触发 WroteHeaders，不触发 WroteRequest
		//     - 有 body 的 POST 请求：先 WroteHeaders 后 WroteRequest
		//   任一事件触发都证明与后端建立了连接。
		var backendCalled bool
		trace := &httptrace.ClientTrace{
			WroteHeaders: func() {
				backendCalled = true
			},
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				backendCalled = true
			},
		}
		// 将 ClientTrace 注入到请求的 context 中，Go 的 http.Transport 会自动触发回调
		clientTraceCtx := httptrace.WithClientTrace(r.Context(), trace)
		// 注意：clientTraceCtx 是 r.Context() 的子节点（带有 trace 回调），
		// 使用 r.WithContext 创建新 Request 而非修改原 Request（Request.WithContext 是浅拷贝）

		// ── 第2步：使用 codeCatcher 包装 ResponseWriter ──
		// codeCatcher 拦截 WriteHeader 调用，记录后端返回的状态码。
		// 因为 Go 的 http.ResponseWriter 接口不提供读取状态码的方法，
		// 只能通过装饰器模式在 WriteHeader 时捕获。
		cc := &codeCatcher{ResponseWriter: w}

		// ── 第3步：执行真实请求 ──
		// 将传入了 clientTraceCtx 的 request 交给下游 handler（通常是反向代理）。
		// handler 内部的 http.Transport 会触发 ClientTrace 回调。
		next.ServeHTTP(cc, r.WithContext(clientTraceCtx))

		// ── 第4步：判断请求是否成功 ──
		// 成功条件（两者必须同时满足）：
		//   1. backendCalled=true：确实与后端建立了连接（排除中间件层面的错误）
		//      如果没有这个条件：路由 404 返回的 404 会被误判为后端返回 4xx
		//
		//   2. cc.statusCode < 500：非服务端错误
		//      500-599 → 服务端内部错误（如代码 panic、数据库挂了）→ 算失败
		//      200-299 → 正常成功 → 算成功
		//      300-399 → 重定向（代理跟随重定向后通常最终拿到的是 200 或 4xx/5xx）
		//      400-499 → 客户端错误（如参数校验失败）→ 不算后端故障，算成功
		//      为什么 4xx 不算失败？被动检查关注的是"后端服务本身是否挂掉"，
		//      4xx 是客户端请求有问题，不是服务故障，不应触发熔断。
		//
		// if backendCalled && cc.statusCode < http.StatusInternalServerError 为 true：
		//   请求成功 → 清空失败记录（滑动窗口重置）
		//
		//   为什么设置 nil 而非 delete？
		//     - 设置 nil：下一次 append(nil, time.Now()) 可以工作（append on nil slice is OK）
		//     - delete：下一次 append 时需要先检查 key 是否存在
		//     设置 nil 更简洁，且避免了 delete + 下一行 append 之间可能创建新 entry 的竞争
		//
		//   注意：statusCode 初始为 0。当后端完全没有响应时（连接被拒、超时等），
		//   codeCatcher.WriteHeader 不会被调用，statusCode=0 < 500，
		//   但此时 backendCalled 通常为 false → 条件不成立 → 进入失败路径（正确）
		//
		// if backendCalled && cc.statusCode < http.StatusInternalServerError 为 false：
		//   至少一个条件不满足 → 进入第5步记录失败
		if backendCalled && cc.statusCode < http.StatusInternalServerError {
			// 请求成功：清空该 target 的失败记录
			// 设置为 nil 而非 delete，是为了利用 nil slice 上 append 可用的特性
			//
			// 为什么在已经熔断的情况下，一次成功就应该清空？
			//   熔断后后端已经被标记为不健康，路由不会再分配流量给它。
			//   但 WrapHandler 包装的是所有请求，包括在熔断前可能已经在 flight 中的请求
			//   （并发场景：SetStatus(false) 和本次成功的请求可能并发发生）。
			//   一次成功清空失败记录是"宽松恢复"策略，允许后端快速恢复流量接收。
			p.failuresMu.Lock()
			p.failures[targetURL] = nil
			p.failuresMu.Unlock()
			return
		}

		// ── 第5步：记录失败 ──
		// 在 failures map 中追加当前时间戳。
		// append(nil, time.Now()) 在 nil slice 上完全合法（Go 语言保证），
		// 会创建一个新的 slice 包含当前时间。
		p.failuresMu.Lock()
		p.failures[targetURL] = append(p.failures[targetURL], time.Now())
		p.failuresMu.Unlock()

		// ── 第6步：判断是否触发熔断 ──
		// healthy() 方法实现了滑动窗口逻辑：
		//   1. 过滤 failureWindow 时间窗口内的失败记录
		//   2. 比较窗口内失败次数和 maxFailedAttempts 阈值
		//
		// if p.healthy(targetURL) 为 true（失败次数 < 阈值）：
		//   尚未达到熔断条件 → 直接返回，不做任何操作
		//
		// if p.healthy(targetURL) 为 false（失败次数 >= 阈值）：
		//   触发熔断 → 进入第7步
		if p.healthy(targetURL) {
			return
		}

		// ── 第7步：触发熔断 ──
		// singleflight.Do 使用 targetURL 作为去重 key：
		//   对同一后端的多个并发失败请求，只有一个 goroutine 真正执行熔断逻辑。
		//   其他并发的 goroutine 会等待这个执行完成（共享返回值），
		//   因为只做一次 SetStatus(false)，之后的调用发现 timer 已存在也会跳过。
		//
		// 返回值 (any, error, shared) 被忽略（_, _, _）：
		//   只有真正执行 fn 的 goroutine 需要设置状态和创建 timer；
		//   被去重的 goroutine 共享结果即可，不需要额外操作。
		_, _, _ = p.timersGroup.Do(targetURL, func() (any, error) {
			// ── 双重防御：检查是否已有恢复 timer 在运行 ──
			// singleflight 已经防止了并发去重（同一时刻只能有一个 goroutine 进来），
			// 但需要处理时序上的情况：
			//   1. 当前 target 被熔断，创建了恢复 timer（30s 后恢复）
			//   2. timer 还在倒计时，又来了新的失败请求
			//   3. singleflight 允许进入（因为之前的调用已经返回）
			//   4. 此时 timers.Load 检测到已有 timer → 跳过重复熔断
			//
			// sync.Map.Load 是无锁读取，性能优于加锁检查。
			//
			// if _, ok := p.timers.Load(targetURL); ok 为 true：
			//   已有恢复 timer 在运行 → 跳过，不重复熔断
			//
			// if _, ok := p.timers.Load(targetURL); ok 为 false：
			//   没有恢复 timer → 继续执行熔断逻辑
			if _, ok := p.timers.Load(targetURL); ok {
				return nil, nil
			}

			// ── 标记后端为不健康 ──
			// 通知负载均衡器将该后端从流量分配中移除。
			// 此后新的请求不会再路由到此后端（除非有其他后端实际可用）。
			p.balancer.SetStatus(ctx, targetURL, false)

			// ── if p.hasActiveHealthCheck 为 true：恢复由主动检查负责 ──
			// 主动检查有自己的恢复逻辑（定时探测发现后端恢复后 SetStatus→true），
			// 被动检查不需要再创建恢复 timer。
			// 这样做的好处：
			//   1. 避免两个恢复机制冲突（被动 timer 恢复 vs 主动检查恢复）
			//   2. 主动检查的探测更可靠（需要后端返回正确的健康状态才恢复）
			//   3. 简化了被动检查的逻辑
			//
			// if p.hasActiveHealthCheck 为 true：
			//   → 只做熔断不做恢复，直接 return
			//
			// if p.hasActiveHealthCheck 为 false：
			//   → 继续创建恢复 timer
			if p.hasActiveHealthCheck {
				return nil, nil
			}

			// ── 创建恢复 timer ──
			// 异步启动一个 goroutine，在 failureWindow 后自动将后端恢复为健康。
			// timer 存储在 p.timers sync.Map 中，用于双重防御检查。
			//
			// 为什么用 goroutine + timer 而不是 time.AfterFunc？
			//   time.AfterFunc 也可以，但这里需要更显式的生命周期管理：
			//   - 需要将 timer 存入 p.timers（Load/Delete 操作）
			//   - 需要在 ctx.Done() 时停止 timer（避免不必要的恢复操作）
			//   封装成一个 goroutine 并使用 select 两条路径，意图更清晰。
			go func() {
				// 创建一个一次性定时器，failureWindow 后触发
				timer := time.NewTimer(time.Duration(p.failureWindow))
				defer timer.Stop() // goroutine 退出时确保 timer 被停止

				// 将 timer 注册到 p.timers 中（sync.Map 的 Store 操作）。
				// Store 之后再到来的熔断请求会发现 timer 存在而跳过。
				// key = targetURL（如 "http://10.0.0.1:8080"）
				p.timers.Store(targetURL, timer)

				// select 二选一：
				//   case <-ctx.Done(): 外部取消（服务关闭/配置刷新）→ 退出，不恢复
				//   case <-timer.C:    failureWindow 时间到 → 执行恢复
				//
				// 两种情况下都退出 goroutine（defer timer.Stop + 函数末尾 return）
				select {
				case <-ctx.Done():
					// ── ctx 被取消（服务关闭或配置刷新） ──
					// 不执行恢复：后端可能已经被新配置替换或移除，恢复没有意义。
					// timer 会在 defer 中被 Stop。
					// 注意：这里没有调用 p.timers.Delete(targetURL)，
					// 因为在 ctx.Done() 的场景下，整个健康检查器即将被释放，
					// 是否有残留不影响后续使用。

				case <-timer.C:
					// ── timer 到期：恢复后端 ──
					// 1. 从 p.timers sync.Map 中删除 timer 记录
					//    （允许后续新的失败请求重新触发熔断逻辑）
					p.timers.Delete(targetURL)
					// 2. 通知负载均衡器后端已恢复
					//    此后新的请求可以再次路由到此后端
					p.balancer.SetStatus(ctx, targetURL, true)
				}
			}()

			return nil, nil
		})

	})
}

// healthy 判断 target 是否仍处于健康状态（失败次数未达到熔断阈值）。
//
// 算法：滑动时间窗口
//
//  1. 计算窗口起点：windowStart = 当前时间 - failureWindow
//  2. 遍历该 target 的失败记录列表（按时间升序排列）
//  3. 找到第一个落在窗口内的失败记录（After(windowStart)），
//     将列表中该记录之前的部分（已过期）丢弃
//  4. 比较窗口内剩余失败次数与 maxFailedAttempts 阈值
//
// 返回值：
//   - true:  失败次数 < maxFailedAttempts，后端仍算健康
//   - false: 失败次数 >= maxFailedAttempts，触发熔断
//
// 并发安全：使用 RLock（读锁），允许多个 goroutine 同时调用 healthy()，
// 仅与写操作（记录失败时间，Lock）互斥。
//
// 使用示例场景：
//
//	// 配置：window=10s, threshold=3
//	// 失败记录：[t-8s, t-5s, t-2s]（3次，都在窗口内）
//	// → windowStart = t-10s
//	// → t-8s.After(t-10s) = true → 保留 [t-8s, t-5s, t-2s] → len=3 >= 3 → return false（熔断！）
//
//	// 失败记录：[t-12s, t-8s]（2次，t-12s 已过期）
//	// → windowStart = t-10s
//	// → t-12s.After(t-10s) = false → 继续
//	// → t-8s.After(t-10s) = true → 保留 [t-8s] → len=1 < 3 → return true（健康）
func (p *PassiveServiceHealthChecker) healthy(targetURL string) bool {
	// 计算滑动窗口的起始时间
	// time.Now() 获取的是 wall clock（墙上时钟），
	// 受 NTP 调整影响但在此场景下影响可以忽略（窗口通常为秒级/分钟级）。
	windowStart := time.Now().Add(-time.Duration(p.failureWindow))

	p.failuresMu.RLock()
	defer p.failuresMu.RUnlock()

	// 获取该 target 的失败记录列表
	failures := p.failures[targetURL]

	// ── 滑动窗口过滤 ──
	// 失败记录是按时间升序排列的（每次都 append 当前时间）。
	// 从左到右扫描，找到第一个在窗口内的记录，丢弃它之前的过期记录。
	//
	// 举例：窗口=10s，记录列表=[t-15s, t-12s, t-5s, t-2s, t-1s]
	// 扫描过程：
	//   i=0: t-15s.After(windowStart=t-10s)? false → 继续（过期）
	//   i=1: t-12s.After(t-10s)? false → 继续（过期）
	//   i=2: t-5s.After(t-10s)? true  → 保留 [t-5s, t-2s, t-1s]（3次失败）
	//   break 跳出循环
	//
	// 为什么用 After 而不是 !Before？
	//   After(t) 等价于 time.After(t) [channel 语义不同]，这里用的是 Time.After。
	//   t.After(windowStart) 等价于 t.UnixNano() > windowStart.UnixNano()
	//   Before 和 After 语义互斥，用 After 更直观：在窗口之后 = 在窗口内。
	//
	// if t.After(windowStart) 为 true：
	//   找到了窗口内第一条记录 → 截取从 i 开始的部分，丢弃之前的过期记录
	//
	// 如果所有记录都是过期的（for 循环完整走完，没有 break）：
	//   failures[i:] 会被越界 panic！
	//   但这种情况不会发生：因为记录是按时间升序排列的，
	//   time.Now() 至少是 >= 最后一条记录的时间（最新记录总是在窗口内），
	//   所以至少最后一条记录一定满足条件。
	for i, t := range failures {
		if t.After(windowStart) {
			// 截取窗口内的记录：丢弃 i 之前的所有过期记录
			// 注意：这里直接修改了 map 中的 slice 引用（而非重新赋值），
			// 因为 RLock 只读保护下可以修改 slice 内容，
			// 但不能修改 map 的 key 结构（那需要 Lock）。
			// 这里通过 failures[targetURL] = failures[i:] 修改的是 map 的 value，
			// 这是一种"在 RLock 下修改共享数据结构"的灰色地带操作。
			// 但由于 failures[i:] 只是截取了同一个底层数组的一部分，
			// 并发读取失败记录时也只会看到部分元素，不会导致数据竞争。
			p.failures[targetURL] = failures[i:]
			break
		}
	}

	// ── 判断是否达到熔断阈值 ──
	// len(p.failures[targetURL])：窗口内剩余失败次数
	// < p.maxFailedAttempts：失败次数小于阈值 → 返回 true（健康）
	// >= p.maxFailedAttempts：失败次数达到阈值 → 返回 false（触发熔断）
	//
	// 举例：
	//   maxFailedAttempts=3, 窗口内有 2 条失败 → 2 < 3 → true（健康）
	//   maxFailedAttempts=3, 窗口内有 3 条失败 → 3 < 3 → false（熔断！）
	return len(p.failures[targetURL]) < p.maxFailedAttempts
}

// ============================================================================
// codeCatcher — HTTP 状态码捕获装饰器
// ============================================================================
//
// codeCatcher 包装 http.ResponseWriter，在 WriteHeader 调用时拦截并记录状态码。
// Go 的 http.ResponseWriter 接口不提供读取状态码的方法，只能通过装饰器模式捕获。
//
// 使用场景：
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    cc := &codeCatcher{ResponseWriter: w} // 包装 w
//	    innerHandler.ServeHTTP(cc, r)          // 传入包装后的 cc
//	    fmt.Println(cc.statusCode)             // 读取内部 handler 设置的状态码
//	}
//
// 还实现了以下接口，确保可以稳定传递到底层 ResponseWriter：
//   - http.Flusher:   将 Flush 调用透传（SSE、流式响应等需要）
//   - http.Hijacker:  将 Hijack 调用透传（WebSocket 升级等需要）
//   - 未实现 http.Pusher: HTTP/2 Server Push 使用较少，且 Go 1.x 已弃用

type codeCatcher struct {
	http.ResponseWriter // 嵌入底层 ResponseWriter，未覆盖的方法（如 Header()）直接透传

	statusCode int // 捕获的状态码，初始为 0
	// 0 的含义："尚未设置"。在 Go 的 http 包中，如果 WriteHeader 从未被显式调用，
	// 第一次 Write 会自动调用 WriteHeader(http.StatusOK)，
	// 所以 codeCatcher.Write 中会检查 statusCode < 200 并假定为 200。
}

// WriteHeader 拦截 WriteHeader 调用，记录状态码后透传到底层。
//
// 允许覆盖：如果后端中间件多次调用 WriteHeader（如先写 103 Early Hints 再写 200），
// 最后一次调用的状态码会被记录，这才是最终实际返回给客户端的状态码。
//
// 示例（多次调用）：
//
//	handler 内部先调用 w.WriteHeader(103) // Early Hints
//	随后调用 w.WriteHeader(200)          // 最终状态码
//	→ codeCatcher.statusCode = 200（最后一次的值）
func (c *codeCatcher) WriteHeader(statusCode int) {
	// 记录最后一次调用的状态码
	c.statusCode = statusCode
	// 透传至底层，确保实际的 http 响应正确发送
	c.ResponseWriter.WriteHeader(statusCode)
}

// Write 拦截 Write 调用。
//
// 根据 Go 的 http.ResponseWriter 文档：如果 WriteHeader 没有被显式调用，
// 第一次 Write 会自动调用 WriteHeader(http.StatusOK)。
// 因此，如果执行到这里的 Write 时 statusCode 仍 < 200（未设置状态码），
// 说明 WriteHeader 未被调用 → 此时假定状态码为 200。
//
// if c.statusCode < http.StatusOK 为 true（statusCode=0 或 1xx）：
//
//	状态码尚未被设置（WriteHeader 未被调用），
//	→ 设置为 200 OK（符合 Go 的 http 包行为）
//	→ 然后正常 Write 数据
//
// if c.statusCode < http.StatusOK 为 false：
//
//	状态码已在 WriteHeader 中设置过 → 直接 Write
//
// 限制：仅设置一次。如果后端在首次 Write 后又改了状态码，
// Write 不会再触发 WriteHeader，所以这里只处理未设置的情况。
func (c *codeCatcher) Write(b []byte) (int, error) {
	// 如果状态码还未设置（初始 0 或 1xx 信息性响应），则假定为 200 OK
	if c.statusCode < http.StatusOK {
		c.statusCode = http.StatusOK
	}

	return c.ResponseWriter.Write(b)
}

// Flush 实现 http.Flusher 接口，将 Flush 调用透传到底层。
//
// Flusher 用于 SSE（Server-Sent Events）、流式响应（chunked encoding）、
// 长轮询等场景，允许 handler 将缓冲区中的数据立即发送给客户端
// 而不等待整个响应完成。
//
// if flusher, ok := c.ResponseWriter.(http.Flusher); ok 为 true：
//
//	底层 ResponseWriter 支持 Flush → 直接透传调用
//
// if ok 为 false：
//
//	底层不支持 Flush → 静默忽略（调用方不应该依赖 Flush 一定成功）
//	如果日志警告可能有大量误导日志
func (c *codeCatcher) Flush() {
	if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 实现 http.Hijacker 接口，从 HTTP 服务器接管底层 TCP 连接。
//
// Hijack 允许 handler 绕过 HTTP 协议层，直接操作原始 TCP 连接。
// 主要场景：
//   - WebSocket 升级：HTTP 握手后切换到 WebSocket 协议
//   - 自定义 TCP 协议：handler 需要直接读写原始 socket
//
// 返回值：
//
//	net.Conn:           原始的 TCP 连接
//	*bufio.ReadWriter:  连接上的带缓冲读写器（可能包含 HTTP 请求已读取但未处理的字节）
//	error:              如果底层不支持 Hijack，返回错误
//
// if h, ok := c.ResponseWriter.(http.Hijacker); ok 为 true：
//
//	底层 ResponseWriter 支持 Hijack → 直接透传，返回原始连接
//
// if ok 为 false：
//
//	底层不支持 Hijack（如 HTTP/2 ResponseWriter）→ 返回错误
//
// 使用注意：
//   - 只有 HTTP/1.x 支持 Hijack，HTTP/2 不支持
//   - 接管连接后必须由调用方负责关闭连接（conn.Close()）
//   - 接管后不能再使用原始的 Request.Body
//   - 接管后的 bufio.ReadWriter 可能包含已读取到缓冲区的数据
func (c *codeCatcher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// 检查底层 ResponseWriter 是否实现了 Hijacker
	if h, ok := c.ResponseWriter.(http.Hijacker); ok {
		// 直接透传，让底层处理连接劫持逻辑
		return h.Hijack()
	}

	// 底层不支持 Hijack → 返回错误（包含类型信息便于排查）
	return nil, nil, fmt.Errorf("not a hijacker: %T", c.ResponseWriter)
}
