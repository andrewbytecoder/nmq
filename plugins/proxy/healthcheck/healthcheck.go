// Package healthcheck 提供主动和被动两种健康检查机制。
//
// 主动健康检查 (ServiceHealthChecker):
//
//	定期向后端发起 HTTP 或 gRPC 探测请求，根据响应更新后端状态。
//	健康的后端和故障的后端使用不同的检查频率，故障后端检查更频繁。
//
// 被动健康检查 (PassiveServiceHealthChecker):
//
//	不主动探测，而是通过包装 HTTP handler 拦截真实请求，
//	统计失败次数做熔断。当失败次数超过阈值时，将后端标记为不健康，
//	经过一个滑动时间窗口后自动恢复。
//
// 两类检查可以共存：主动检查负责恢复状态，被动检查负责快速熔断。
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
// 健康检查组件本身不维护后端状态（健康/故障），而是通过该接口通知上层组件（如负载均衡器）。
// 服务（如负载均衡器）应该实现此接口，接收健康检查结果并据此调整流量分配。
type StatusSetter interface {
	// SetStatus 设置指定子服务（后端）的健康状态。
	// childName: 子服务的唯一标识，通常是 target 的 name（不是 URL）。
	// status: true 表示健康可用，false 表示故障不可用。
	SetStatus(ctx context.Context, childName string, status bool)
}

// StatusUpdater 状态传播接口。
// 当某个服务自身状态发生变化（例如它的所有子节点都挂了），
// 需要将状态变化向上传播给父级服务。实现此接口的服务可以注册一个回调函数，
// 当自身状态变化时触发通知。
type StatusUpdater interface {
	RegisterStatusUpdater(fn func(up bool)) error
}

// target 表示一个健康检查目标（后端服务）。
// name 是该后端的逻辑名称（如 "server1"），用于状态通知。
// targetUrl 是该后端的完整 URL（如 "http://10.0.0.1:8080"），用于发起探测请求。
type target struct {
	name      string
	targetUrl *url.URL
}

// ServiceHealthChecker 主动健康检查器。
// 通过定时器定期向所有后端发起探测请求（HTTP 或 gRPC），
// 根据响应结果更新后端状态。健康的后端以正常频率检查，
// 故障的后端以更快的频率检查（unhealthyInterval），以便及时发现恢复。
type ServiceHealthChecker struct {
	ctx          context.Context     // 生命周期上下文，ctx 取消时健康检查停止
	log          *zap.Logger         // 日志记录器
	roundTripper http.RoundTripper   // HTTP 传输层，用于复用连接池等底层配置
	targets      map[string]*url.URL // 所有待检查的后端，key 为名称，value 为 URL

	balancer StatusSetter            // 状态通知接口，检查结果通过它通知负载均衡器
	info     *runtimecfg.ServiceInfo // 运行时服务信息，用于更新后端状态快照

	config            *dynamic.ServerHealthCheck // 健康检查配置（路径、方法、超时等）
	interval          time.Duration              // 健康后端的检查间隔
	unhealthyInterval time.Duration              // 故障后端的检查间隔（通常比 interval 短）
	timeout           time.Duration              // 单次健康检查的超时时间

	client *http.Client // HTTP 客户端，用于发起探测请求

	healthyTargets   chan target // 健康后端队列：当前健康的后端放入此 channel，按 interval 频率取出检查
	unhealthyTargets chan target // 故障后端队列：当前故障的后端放入此 channel，按 unhealthyInterval 频率取出检查

	serviceName string // 服务名称，用于日志标识
}

// ============================================================================
// ServiceHealthChecker 的 Options 模式构造器
// 使用 functional options 模式，每个 Set* 函数返回一个 options.Option，
// 在 NewServiceHealthChecker 中依次应用到实例上。
// ============================================================================

// SetServiceName 设置服务名称。
func SetServiceName(serviceName string) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.serviceName = serviceName
	}
}

// SetBalancer 设置状态通知接口。
func SetBalancer(balancer StatusSetter) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.balancer = balancer
	}
}

// SetInfo 设置运行时服务信息。
func SetInfo(info *runtimecfg.ServiceInfo) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.info = info
	}
}

// SetConfig 设置健康检查配置（路径、方法、超时等）。
func SetConfig(config *dynamic.ServerHealthCheck) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.config = config
	}
}

// SetRoundTripper 设置 HTTP 传输层。
func SetRoundTripper(rp http.RoundTripper) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.roundTripper = rp
	}
}

// SetLogger 设置日志记录器。
func SetLogger(log *zap.Logger) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.log = log
	}
}

// SetCtx 设置生命周期上下文，ctx 取消时所有健康检查停止。
func SetCtx(ctx context.Context) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.ctx = ctx
	}
}

// SetTargets 设置所有待检查的后端。
func SetTargets(targets map[string]*url.URL) options.Option {
	return func(c any) {
		cfg := c.(*ServiceHealthChecker)
		cfg.targets = targets
	}
}

// NewServiceHealthChecker 创建主动健康检查器。
// 使用 functional options 模式注入所有依赖，然后校验和补齐配置：
//  1. 检查间隔 (interval): 未配置或 <= 0 时使用默认值
//  2. 故障检查间隔 (unhealthyInterval): 未配置时与 interval 相同，即一视同仁
//  3. 超时时间 (timeout): 未配置或 <= 0 时使用默认值
//  4. 重定向策略: 配置了 FollowRedirects=false 时，禁用 HTTP 客户端自动跟随重定向，
//     因为重定向后的目标健康不代表原始目标健康
//  5. 初始化两个 channel: 所有后端初始都是健康的，放入 healthyTargets；unhealthyTargets 初始为空
//
// 返回配置完毕但尚未启动的健康检查器。
func NewServiceHealthChecker(opts ...options.Option) *ServiceHealthChecker {
	shc := &ServiceHealthChecker{}
	// 依次应用所有 options
	for _, opt := range opts {
		opt(shc)
	}

	// ── Interval 校验 ──
	// 健康后端的检查间隔，控制多久对状态为 UP 的后端发起一次探测
	interval := time.Duration(shc.config.Interval)
	if interval <= 0 {
		shc.log.Warn("Health check interval smaller than zero, default value will be used instead.")
		interval = time.Duration(dynamic.DefaultHealthCheckInterval)
	}

	// ── UnhealthyInterval 校验 ──
	// 故障后端的检查间隔，通常设置得比 interval 短，以便更快发现后端恢复
	// 如果未配置 UnhealthyInterval（为 nil），则与 interval 相同，健康/故障使用统一频率
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
	// 单次健康检查请求的超时时间，防止后端不响应导致协程泄漏
	timeout := time.Duration(shc.config.Timeout)
	if timeout <= 0 {
		shc.log.Warn("Health check timeout smaller than zero, default value will be used instead.")
		timeout = time.Duration(dynamic.DefaultHealthCheckTimeout)
	}

	// ── HTTP 客户端 ──
	// 使用独立的 http.Client，通过 roundTripper 复用连接池配置
	client := &http.Client{
		Transport: shc.roundTripper,
	}

	// ── 重定向策略 ──
	// Go 的 http.Client 默认自动跟随重定向（最多 10 次）。
	// 但健康检查要探测的是目标本身的状态，不是重定向后的目标。
	// 示例场景：后端 A 返回 302 跳转到后端 B，B 返回 200，
	// 如果跟随重定向，会误判 A 是健康的，而实际上 A 可能已经不可用。
	// 因此当配置 FollowRedirects=false 时，通过 CheckRedirect 回调返回
	// http.ErrUseLastResponse，告诉客户端把重定向响应本身当作最终结果返回。
	if shc.config.FollowRedirects != nil && !*shc.config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// ── 初始化 channel ──
	// 创建带缓冲的 channel，缓冲大小等于后端数量，避免发送阻塞。
	// 初始状态：所有后端都是健康的，放入 healthyTargets channel。
	// unhealthyTargets 初始为空，等待后续有后端变故障时才会被填入。
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
func (shc *ServiceHealthChecker) Launch(ctx context.Context) {
	shc.log.Info("Launching unhealthy checker", zap.String("service", shc.serviceName))
	// 故障检查协程异步启动，不阻塞 Launch 主流程
	go shc.healthCheck(ctx, shc.unhealthyTargets, shc.unhealthyInterval)

	shc.log.Info("Launching healthy checker", zap.String("service", shc.serviceName))
	// 健康检查在主协程中阻塞运行，直到 ctx 取消
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
func (shc *ServiceHealthChecker) healthCheck(ctx context.Context, targets chan target, interval time.Duration) {
	// 创建定时器，每隔 interval 时间触发一次检查
	ticker := time.NewTicker(interval)
	defer ticker.Stop() // 函数退出时释放 ticker 资源

	for {
		select {
		case <-ctx.Done():
			// ctx 取消时（服务关闭或配置刷新）退出循环
			return

		case <-ticker.C:
			// ── 第一层：批量收集当前所有待检查的 target ──
			// 使用 for + select + default 模式，非阻塞地一次性取出 channel 中
			// 所有立即可用的 target，攒够了就统一处理，不等空 channel 阻塞。
			//
			// 为什么用 hasMoreTargets 布尔变量而不是 break？
			// 因为 select 中的 break 只能跳出 select 本身，不能跳出外层 for 循环。
			// 如果不使用标签 break 或 bool 变量，break 只会退出 select，
			// 然后回到 for 开头继续 select → default → break → 死循环。
			var targetsToCheck []target
			hasMoreTargets := true
			for hasMoreTargets {
				select {
				case <-ctx.Done():
					return
				case t := <-targets:
					// channel 中有数据，收集到切片中
					targetsToCheck = append(targetsToCheck, t)
				default:
					// channel 中没有立即可用的数据了，停止收集
					hasMoreTargets = false
				}
			}

			// ── 第二层：逐一对收集到的 target 执行健康检查 ──
			for _, t := range targetsToCheck {
				// 在每次健康检查前，非阻塞地检查 ctx 是否已取消。
				//
				// 为什么需要空 default？
				// 如果去掉 default，select 只有一个 case <-ctx.Done()，
				// 当 ctx 未取消时，select 会永久阻塞在这个 case 上，
				// 导致后面的 executeHealthCheck 永远执行不到 → 整个循环卡死。
				// 空 default 让 select "不阻塞"：ctx 取消了就 return，没取消就继续往下走。
				select {
				case <-ctx.Done():
					return
				default:
					// ctx 尚未取消，跳过，继续执行健康检查
				}

				up := true
				// 执行实际的健康检查探测（HTTP 或 gRPC）
				if err := shc.executeHealthCheck(ctx, shc.config, t.targetUrl); err != nil {
					// 如果错误是 context.Canceled，说明是配置刷新触发的 ctx 取消，
					// 此时不需要记录错误日志，直接退出
					if errors.Is(err, context.Canceled) {
						return
					}

					shc.log.Error("Health check failed", zap.String("service", shc.serviceName), zap.String("target", t.name), zap.Error(err))
					up = false
				}

				// ── 第1步：通知负载均衡器状态变化 ──
				shc.balancer.SetStatus(ctx, t.name, up)

				// ── 第2步：根据结果将 target 路由到正确的 channel ──
				// 这是双 channel 模式的核心：target 在两个 channel 之间流转
				var statusStr string
				if up {
					// 后端健康 → 放回 healthyTargets（下次按 interval 频率检查）
					statusStr = runtimecfg.StatusUp
					shc.healthyTargets <- t
				} else {
					// 后端故障 → 放入 unhealthyTargets（下次按 unhealthyInterval 频率检查）
					statusStr = runtimecfg.StatusDown
					shc.unhealthyTargets <- t
				}

				// ── 第3步：更新运行时状态快照 ──
				shc.info.UpdateServerStatus(t.targetUrl.String(), statusStr)

				shc.log.Info("Health check result", zap.String("service", shc.serviceName), zap.String("target", t.name), zap.String("status", statusStr))
			}
		}
	}
}

const modeGRPC = "grpc" // gRPC 模式标识，与 HTTP 默认模式区分

// executeHealthCheck 执行单次健康检查，根据配置的 Mode 分发到 HTTP 或 gRPC 检查。
//
// 每次检查前创建一个带 deadline 的子 context：
//   - deadline = 当前时间 + timeout
//   - deadline 到达时，进行中的 HTTP/gRPC 请求会被自动取消，避免协程泄漏
//   - defer cancel() 确保子 context 在函数返回时被释放，防止 context 泄漏
func (shc *ServiceHealthChecker) executeHealthCheck(ctx context.Context, config *dynamic.ServerHealthCheck, target *url.URL) error {
	// 创建带截止时间的子 context，超时后请求自动取消
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(shc.timeout))
	defer cancel()

	switch config.Mode {
	case modeGRPC:
		// gRPC 健康检查：使用 gRPC Health Checking Protocol v1
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
// 判断逻辑：
//  1. config.Status == 0（未配置期望状态码）：
//     使用默认规则：2xx 和 3xx 视为健康，4xx 及以上视为故障
//  2. config.Status != 0（配置了期望状态码）：
//     严格匹配：只有响应状态码 == 配置值才视为健康
//
// 注意：defer resp.Body.Close() 确保响应体被关闭，否则会导致连接泄漏。
func (shc *ServiceHealthChecker) checkHealthHTTP(ctx context.Context, target *url.URL) error {
	// 构造健康检查 HTTP 请求（路径、端口、scheme、header 等由 newRequest 处理）
	req, err := shc.newRequest(ctx, target)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 发起 HTTP 请求。shc.client 可能已配置了不跟随重定向的策略
	resp, err := shc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close() // 必须关闭响应体，否则连接无法复用

	// ── 判断健康状态 ──
	// config.Status == 0：未配置期望值，使用默认规则
	//   - 200-399 (2xx, 3xx)：健康
	//   - 400+ (4xx, 5xx)：故障
	//   - 为什么把 3xx 也视为健康：3xx 是重定向，说明目标服务在正常运行，
	//     只是指示客户端去另一个地址，这不代表服务本身有问题
	if shc.config.Status == 0 && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest) {
		return fmt.Errorf("received error status code: %v", resp.StatusCode)
	}

	// config.Status != 0：配置了期望值，必须精确匹配
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
// 流程：
//  1. 确定目标地址（端口优先使用 config.Port，否则用 serverURL 的端口）
//  2. 根据 scheme 决定是否使用 insecure 连接（h2c 表示明文 HTTP/2）
//  3. 调用 gRPC Health.Check 方法
//  4. 根据响应状态码判断健康状态（SERVING=健康，其他=故障）
//
// 错误处理：
//   - codes.Unimplemented: gRPC 服务未实现健康检查协议
//   - codes.DeadlineExceeded: 连接或检查超时（受 executeHealthCheck 中的 timeout 控制）
//   - codes.Canceled: context 已被取消（配置刷新导致）
func (shc *ServiceHealthChecker) checkHealthGRPC(ctx context.Context, serverURL *url.URL) error {
	// ── 确定端口 ──
	// 优先使用健康检查配置中指定的 Port，未配置则使用 serverURL 自带的端口
	port := serverURL.Port()
	if shc.config.Port != 0 {
		port = strconv.Itoa(shc.config.Port)
	}

	// 拼接地址：hostname:port，JoinHostPort 自动处理 IPv6 方括号
	serverAddr := net.JoinHostPort(serverURL.Hostname(), port)

	// ── 配置 gRPC 连接选项 ──
	// 对于 http、h2c（明文 HTTP/2）或空 scheme，使用 insecure 连接
	// 否则默认使用 TLS（生产环境）
	var opts []grpc.DialOption
	switch shc.config.Scheme {
	case "http", "h2c", "":
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// ── 创建 gRPC 连接 ──
	// DialContext 会阻塞直到连接建立或超时（由 ctx 的 deadline 控制）
	conn, err := grpc.DialContext(ctx, serverAddr, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("fail to connect to %s within %s: %w", serverAddr, shc.config.Timeout, err)
		}
		return fmt.Errorf("fail to connect to %s: %w", serverAddr, err)
	}
	defer func() { _ = conn.Close() }() // 确保连接被关闭

	// ── 调用 gRPC Health Check API ──
	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		// 使用 grpc/status 包解析 gRPC 状态码，提供更精确的错误信息
		if stat, ok := status.FromError(err); ok {
			switch stat.Code() {
			case codes.Unimplemented:
				// gRPC 服务没有实现 Health 协议 → 视为故障
				return fmt.Errorf("gRPC server does not implement the health protocol: %w", err)
			case codes.DeadlineExceeded:
				// 检查超时
				return fmt.Errorf("gRPC health check timeout: %w", err)
			case codes.Canceled:
				// Context 被取消（通常是外部触发的配置刷新）
				return context.Canceled
			}
		}

		return fmt.Errorf("gRPC health check failed: %w", err)
	}

	// ── 判断健康状态 ──
	// 只有 ServingStatus == SERVING 才代表健康
	// 其他值（NOT_SERVING, UNKNOWN, SERVICE_UNKNOWN 等）均视为不健康
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
	balancer    StatusSetter // 状态通知接口

	maxFailedAttempts    int            // 滑动窗口内允许的最大失败次数（阈值）
	failureWindow        types.Duration // 失败计数和熔断恢复的滑动时间窗口
	hasActiveHealthCheck bool           // 是否同时配置了主动健康检查

	failuresMu sync.RWMutex           // 保护 failures map 的读写锁
	failures   map[string][]time.Time // key=targetURL, value=失败时间列表（滑动窗口内）

	timersGroup singleflight.Group // 防止并发场景下对同一 target 重复执行熔断逻辑
	timers      sync.Map           // key=targetURL, value=*time.Timer（恢复定时器）
}

// ============================================================================
// PassiveServiceHealthChecker 的 Options 模式构造器
// ============================================================================

// SetPassiveServiceName 设置服务名称。
func SetPassiveServiceName(serviceName string) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.serviceName = serviceName
	}
}

// SetPassiveBalancer 设置状态通知接口。
func SetPassiveBalancer(balancer StatusSetter) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.balancer = balancer
	}
}

// SetPassiveMaxFailedAttempts 设置熔断阈值：滑动窗口内最大失败次数。
func SetPassiveMaxFailedAttempts(maxFailedAttempts int) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.maxFailedAttempts = maxFailedAttempts
	}
}

// SetPassiveFailureWindow 设置滑动时间窗口：失败计数和熔断恢复的时间范围。
func SetPassiveFailureWindow(failureWindow types.Duration) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.failureWindow = failureWindow
	}
}

// SetPassiveHasActiveHealthCheck 标记是否同时配置了主动健康检查。
// 如果为 true，被动检查只负责熔断（SetStatus→false），恢复由主动检查负责。
func SetPassiveHasActiveHealthCheck(hasActiveHealthCheck bool) options.Option {
	return func(o any) {
		shc := o.(*PassiveServiceHealthChecker)
		shc.hasActiveHealthCheck = hasActiveHealthCheck
	}
}

// NewPassiveHealthChecker 创建被动健康检查器。
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
//	ctx:  生命周期上下文，用于控制 timer goroutine 的生命周期
//	next: 实际的 HTTP handler（如反向代理）
//	targetURL: 后端地址，同时作为 failutes map 和 timers sync.Map 的 key
func (p *PassiveServiceHealthChecker) WrapHandler(ctx context.Context, next http.Handler, targetURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ── 第1步：安装 ClientTrace 检测是否真的与后端通信 ──
		// httptrace.ClientTrace 是 Go 标准库提供的 HTTP 客户端事件追踪机制。
		// 它会在各种网络事件发生时触发回调（DNS 解析、TCP 握手、写请求等）。
		//
		// 这里监听两个写入事件：
		//   - WroteHeaders: 请求头已写入 TCP 连接
		//   - WroteRequest:  整个请求体已写入 TCP 连接
		//
		// backendCalled=true 意味着请求确实发送到了后端，
		// 而非被中间件（如路由不匹配）直接返回错误。
		// 这样可以区分"后端故障"和"没有后端可用"两种情况。
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

		// ── 第2步：使用 codeCatcher 包装 ResponseWriter ──
		// codeCatcher 拦截 WriteHeader 调用，记录后端返回的状态码。
		// 因为 Go 的 http.ResponseWriter 接口不提供读取状态码的方法，
		// 只能通过装饰器模式在 WriteHeader 时捕获。
		cc := &codeCatcher{ResponseWriter: w}

		// ── 第3步：执行真实请求 ──
		next.ServeHTTP(cc, r.WithContext(clientTraceCtx))

		// ── 第4步：判断请求是否成功 ──
		// 成功条件：
		//   1. backendCalled=true：确实与后端建立了连接（排除客户端层面错误）
		//   2. statusCode < 500：非服务端错误（2xx 正常，3xx 重定向，4xx 客户端错误）
		//
		// 注意：statusCode 初始为 0。当后端完全没有响应时（连接被拒、超时等），
		// codeCatcher.WriteHeader 不会被调用，statusCode=0 < 200，
		// 加上 backendCalled 此时通常为 false，会被正确识别为失败。
		if backendCalled && cc.statusCode < http.StatusInternalServerError {
			// 请求成功：清空该 target 的失败记录
			// 设置为 nil 而非 delete，是为了利用 append 在 nil slice 上也可用
			p.failuresMu.Lock()
			p.failures[targetURL] = nil
			p.failuresMu.Unlock()
			return
		}

		// ── 第5步：记录失败 ──
		// 在 failures map 中追加当前时间戳
		// appending to nil slice is safe in Go
		p.failuresMu.Lock()
		p.failures[targetURL] = append(p.failures[targetURL], time.Now())
		p.failuresMu.Unlock()

		// ── 第6步：判断是否触发熔断 ──
		// healthy() 方法实现了滑动窗口逻辑：
		//   1. 过滤 failureWindow 时间窗口内的失败记录
		//   2. 比较窗口内失败次数和 maxFailedAttempts 阈值
		// 如果失败次数在阈值内（healthy=true），不做处理
		if p.healthy(targetURL) {
			return
		}

		// ── 第7步：触发熔断 ──
		// singleflight.Do 使用 targetURL 作为去重 key，
		// 确保对同一后端并发触发的多次熔断只会执行一次状态更新。
		// 不同 targetURL 的调用可以并发执行（key 不同）。
		//
		// 返回值 (any, error, shared) 被忽略（_, _, _），
		// 因为只有真正执行 fn 的 goroutine 需要设置状态和创建 timer；
		// 被去重的 goroutine 不需要做任何额外操作。
		_, _, _ = p.timersGroup.Do(targetURL, func() (any, error) {
			// ── 双重防御：检查是否已有恢复 timer 在运行 ──
			// singleflight 已经防止了并发去重，但场景可能出现在：
			//   当前 target 的恢复 timer 还没到期，又收到了新的失败请求。
			// 此时不需要再次设置状态为 false（已经是 false）和创建新 timer。
			// 使用 sync.Map.Load 是线程安全的无锁读取。
			if _, ok := p.timers.Load(targetURL); ok {
				return nil, nil
			}

			// ── 标记后端为不健康 ──
			p.balancer.SetStatus(ctx, targetURL, false)

			// ── 如果配置了主动健康检查，恢复由其负责 ──
			// 主动检查有自己的恢复逻辑（定时探测发现后端恢复后 SetStatus→true），
			// 被动检查不需要再创建恢复 timer，避免与主动检查冲突。
			if p.hasActiveHealthCheck {
				return nil, nil
			}

			// ── 创建恢复 timer ──
			// 异步启动一个 goroutine，在 failureWindow 后自动将后端恢复为健康。
			// timer 存储在 p.timers sync.Map 中，用于上面的双重防御检查。
			go func() {
				// 创建一个一次性定时器，failureWindow 后触发
				timer := time.NewTimer(time.Duration(p.failureWindow))
				defer timer.Stop() // goroutine 退出时确保 timer 被停止

				// 将 timer 注册到 p.timers 中
				// Store 之后再到来的熔断请求会发现 timer 存在而跳过
				p.timers.Store(targetURL, timer)

				select {
				case <-ctx.Done():
					// ctx 被取消（服务关闭或配置刷新），不执行恢复
				case <-timer.C:
					// timer 到期：恢复后端
					// 1. 从 p.timers 中删除 timer 记录
					p.timers.Delete(targetURL)
					// 2. 通知负载均衡器后端已恢复
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
// 并发安全：使用 RLock（读锁），允许多个 goroutine 同时判断，
// 仅与写操作（记录失败时间）互斥。
func (p *PassiveServiceHealthChecker) healthy(targetURL string) bool {
	// 计算滑动窗口的起始时间
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
	// 扫描：
	//   t-15s: After(windowStart=t-10s)? false → 继续
	//   t-12s: After(t-10s)? false → 继续
	//   t-5s:  After(t-10s)? true  → 保留 [t-5s, t-2s, t-1s]（3次失败）
	//
	// 注意：这里修改了共享的 failures slice 的引用，
	// 但通过 RLock 保护，并发写入时会被阻塞。
	for i, t := range failures {
		if t.After(windowStart) {
			p.failures[targetURL] = failures[i:]
			break
		}
	}

	// ── 判断是否达到熔断阈值 ──
	// 窗口内失败次数 >= maxFailedAttempts 则触发熔断（return false）
	return len(p.failures[targetURL]) < p.maxFailedAttempts
}

// ============================================================================
// codeCatcher — HTTP 状态码捕获装饰器
// ============================================================================
//
// codeCatcher 包装 http.ResponseWriter，在 WriteHeader 调用时拦截并记录状态码。
// Go 的 http.ResponseWriter 接口不提供读取状态码的方法，只能通过装饰器模式捕获。
//
// 还实现了：
//   - http.Flusher: 将 Flush 调用透传到底层 ResponseWriter
//   - http.Hijacker: 将 Hijack 调用透传（WebSocket 升级等场景需要）

type codeCatcher struct {
	http.ResponseWriter // 嵌入底层 ResponseWriter，未覆盖的方法（如 Header()）直接透传

	statusCode int // 捕获的状态码，初始为 0
}

// WriteHeader 拦截 WriteHeader 调用，记录状态码后透传到底层。
//
// 允许覆盖：如果后端中间件多次调用 WriteHeader（如先写 103 Early Hints 再写 200），
// 最后一次调用的状态码会被记录，这才是最终实际返回给客户端的状态码。
func (c *codeCatcher) WriteHeader(statusCode int) {
	// 记录最后一次调用的状态码
	c.statusCode = statusCode
	// 将调用透传到底层 ResponseWriter
	c.ResponseWriter.WriteHeader(statusCode)
}

// Write 拦截 Write 调用。
//
// 根据 Go 的 http.ResponseWriter 文档：如果 WriteHeader 没有被显式调用，
// 第一次 Write 会自动调用 WriteHeader(http.StatusOK)。
// 因此，如果执行到 Write 时 statusCode 仍 < 200（未设置），
// 说明 WriteHeader 未被调用，此时假定状态码为 200。
//
// 限制：仅设置一次。如果后端在首次 Write 后又改了状态码，
// Write 不会再触发 WriteHeader，所以这里只处理未设置的情况。
func (c *codeCatcher) Write(b []byte) (int, error) {
	// 如果状态码还未设置（初始 0 或 1xx），则假定为 200 OK
	if c.statusCode < http.StatusOK {
		c.statusCode = http.StatusOK
	}

	return c.ResponseWriter.Write(b)
}

// Flush 实现 http.Flusher 接口，将 Flush 调用透传到底层。
//
// Flusher 用于 SSE（Server-Sent Events）、流式响应等场景，
// 允许 handler 将缓冲区中的数据立即发送给客户端而不等待整个响应完成。
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
// 注意：
//   - 只有 HTTP/1.x 支持 Hijack，HTTP/2 不支持
//   - 接管后必须由调用方负责关闭连接
//   - 接管后不能再使用原始的 Request.Body
func (c *codeCatcher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// 检查底层 ResponseWriter 是否实现了 Hijacker
	if h, ok := c.ResponseWriter.(http.Hijacker); ok {
		// 直接透传，让底层处理连接劫持逻辑
		return h.Hijack()
	}

	return nil, nil, fmt.Errorf("not a hijacker: %T", c.ResponseWriter)
}
