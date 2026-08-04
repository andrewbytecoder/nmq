package tcp

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"time"

	tcpmuxer "github.com/andrewbytecoder/nmq/plugins/proxy/muxer/tcp"
	"github.com/andrewbytecoder/nmq/plugins/proxy/tcp"
	nmqtls "github.com/andrewbytecoder/nmq/plugins/proxy/tls"
	"github.com/go-acme/lego/v5/challenge/tlsalpn01"
	"go.uber.org/zap"
)

// errClientHelloRead 是哨兵错误（sentinel error），用于在读取到 TLS ClientHello 后
// 中断 TLS 握手流程，从而在不消耗额外字节的情况下获取 SNI 和 ALPN 信息。
//
// clientHelloInfo 函数中：启动一次假的 TLS 握手 → GetConfigForClient 回调中
// 读取到 ClientHello 后返回 (nil, errClientHelloRead) → Handshake() 收到此错误后停止 →
// 调用方通过 errors.Is 判断是否为预期内的终止。
//
// 使用示例（见 clientHelloInfo）：
//
//	server := tls.Server(readOnlyConn{conn: conn}, &tls.Config{
//	    GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
//	        sni = hello.ServerName       // 捕获 SNI
//	        protos = hello.SupportedProtos // 捕获 ALPN 协议列表
//	        return nil, errClientHelloRead // 中断握手
//	    },
//	})
//	if err := server.Handshake(); !errors.Is(err, errClientHelloRead) {
//	    // 如果错误不是 errClientHelloRead → 真正的 TLS 握手失败
//	}
//	// 错误是 errClientHelloRead → 成功读取了 ClientHello 信息
var errClientHelloRead = errors.New("client hello successfully read")

// tlsConfigWithOptionsName 将 TLS 配置与其对应的 TLSOption 名称绑定在一起。
//
// 由于同一个 SNI 主机的不同 provider 可能提供不同的 TLS 配置，
// optionsName 用于追踪当前生效的是哪个 TLSOption。
//
// 字段说明：
//
//	cfg:         该 SNI 主机对应的 TLS 配置（证书、曲线、加密套件等）
//	             nil 表示 TLS 配置损坏或缺失，将使用 brokenTLSRouter 关闭连接
//	optionsName: 配置来源名称（如 "default"），用于标识和日志
//
// 使用场景：
//
//	通过 AddHTTPTLSConfig 添加：hostHTTPTLSConfig["example.com"] = tlsConfigWithOptionsName{cfg, "my-tls-option"}
//	通过 SetHTTPSForwarder 消费：为每个 SNI 创建 TLSHandler(tlsConf.cfg, tlsConf.optionsName)
type tlsConfigWithOptionsName struct {
	cfg         *tls.Config
	optionsName string
}

// Router 是 TCP 协议路由器，负责根据协议类型（纯 TCP、HTTP、HTTPS、Postgres、TLS）分发连接。
//
// 核心设计：三层 Muxer 架构
//
//	┌───────────── 新连接进入 ─────────────┐
//	│                                      │
//	│  ┌── peek 首字节, 协议检测 ──┐         │
//	│  │  Postgres? TLS? HTTP?   │         │
//	│  └─────────────────────────┘         │
//	│                                      │
//	│  ┌──── muxerTCP ─────┐  纯 TCP 路由   │
//	│  ├──── muxerTCPTLS ──┤  TCP-TLS 路由  │
//	│  ├──── muxerHTTPS ───┤  HTTPS 路由    │
//	│  └───────────────────┘               │
//	│                                      │
//	│  ┌── Fallback: httpForwarder ───┐     │
//	│  ├── Fallback: httpsForwarder ──┤     │
//	│  └──────────────────────────────┘     │
//
// 路由优先级：HTTPS(HostSNI 精确匹配) > TCP-TLS(HostSNI 精确匹配) > HTTPS catchall > TCP-TLS catchall > httpsForwarder
//
// 使用示例（典型初始化流程）：
//
//	// 1. 创建 Router
//	router, _ := tcp.NewRouter(logger, []string{"file", "kubernetes"})
//
//	// 2. 添加路由
//	router.AddTCPRoute("HostSNI(`example.com`)", 10, "file", myTCPHandler)
//	router.AddHTTPTLSConfig("example.com", myTLSConfig, "my-tls-option")
//
//	// 3. 设置 HTTP 转发器（处理 HTTP/HTTPS 请求）
//	router.SetHTTPForwarder(httpTCPForwarder)
//	router.SetHTTPSForwarder(httpsTCPForwarder)
//
//	// 4. 设置 HTTP handler（用于 Switcher 在配置重新加载时替换）
//	router.SetHTTPHandler(myHTTPMux)
//	router.SetHTTPSHandler(myHTTPSMux)
//
//	// 5. 启动 ACME TLS 直通（可选）
//	router.EnableACMETLSPassthrough(true)
//
//	// 6. 在 TCP 服务器中使用
//	tcpServer.OnConnection(func(conn tcp.WriteCloser) {
//	    router.ServeTCP(conn)
//	})
type Router struct {
	log                *zap.Logger // 日志记录器
	acmeTLSPassthrough bool        // 是否启用 ACME-TLS/1 挑战直通

	// ── 三层 Muxer（路由表）──
	// Contains TCP routes.
	// 纯 TCP 路由：不涉及 TLS，直接匹配 HostSNI 规则
	muxerTCP tcpmuxer.Muxer

	// Contains TCP TLS routes.
	// TCP-TLS 路由：匹配 TLS 连接后使用特定 TLS 配置
	// 支持 TLS Passthrough（原封不动转发 TLS 流量到后端）
	muxerTCPTLS tcpmuxer.Muxer

	// Contains HTTPS routes.
	// HTTPS 路由：匹配后解密 TLS，将解密后的 HTTP 流量交给 httpsForwarder
	// 每个路由条目都绑定了特定的 TLS 配置（证书等）
	muxerHTTPS tcpmuxer.Muxer

	// Forwarder handlers.
	// httpForwarder 处理所有 HTTP 明文请求（非 TLS 的 HTTP/1.x）。
	// 当 muxerTCP 未匹配到路由（但连接确实是 HTTP 时）作为 fallback。
	// 通常实现为一个 HTTP 服务器，解析 HTTP 请求后转发到后端。
	httpForwarder tcp.Handler

	// httpsForwarder 处理所有 HTTPS 请求（经过 TLS 解密后）。
	// 两种使用路径：
	//   1. 间接：muxerHTTPS 匹配到路由 → 返回的 TLSHandler 使用特定证书解密 → 交给 httpsForwarder
	//   2. 直接：作为最后的 fallback（所有路由都未匹配），使用默认证书解密后交给 httpsForwarder
	httpsForwarder tcp.Handler

	// Neither is used directly, but they are held here, and recreated on config reload,
	// so that they can be passed to the Switcher at the end of the config reload phase.
	// httpHandler/httpsHandler 不直接在 Router 内部使用，而是在配置重新加载时
	// 由 Switcher 替换。因为配置变更时旧的 handler 可能还有进行中的请求，
	// 需要"旧 handler 继续服务旧连接，新 handler 服务新连接"的平滑过渡。
	// 这两个字段保存最新的 handler 引用，供 Switcher 在下一个重新加载周期使用。
	httpHandler  http.Handler
	httpsHandler http.Handler

	// TLS configs.
	// httpsTLSConfig 是默认 TLS 配置。
	// 当 muxerHTTPS 和 muxerTCPTLS 都无法匹配到特定 SNI 的配置时，
	// 使用此默认配置进行 TLS 握手。如果为 nil，回退到 brokenTLSRouter（直接关闭连接）。
	httpsTLSConfig *tls.Config // default TLS config

	// hostHTTPTLSConfig 存储 SNI → TLS配置 的映射。
	// key: SNI 主机名（如 "example.com"），由 AddHTTPTLSConfig 添加
	// value: tlsConfigWithOptionsName（TLS配置 + 来源名称）
	// nil config 是设置 brokenTLSRouter 的线索（表示该 SNI 的 TLS 配置损坏/不可用）
	hostHTTPTLSConfig map[string]tlsConfigWithOptionsName // TLS configs keyed by SNI
}

// NewRouter 创建一个新的 TCP 路由器。
//
// 同时初始化三层 Muxer（muxerTCP、muxerTCPTLS、muxerHTTPS），
// 每层独立管理自己的路由表和匹配规则。
//
// 参数说明：
//
//	log:                日志记录器
//	providerPrecedence: provider 名称列表，按优先级从高到低排列
//	                    例如 ["file", "kubernetes", "consul"]，
//	                    当多个 provider 为同一规则注册路由时，
//	                    优先级高的 provider 的路由生效（tie-breaking）
//
// 使用示例：
//
//	// 文件系统中的配置优先级最高，其次是 Kubernetes
//	router, err := tcp.NewRouter(logger, []string{"file", "kubernetes"})
//	if err != nil {
//	    log.Fatal("Failed to create TCP router:", err)
//	}
//
// 返回错误场景：
//
//	任一 muxer 创建失败（通常是 providerPrecedence 参数不合理导致）
//
// 返回后 router 处于初始化状态：
//   - 三层 muxer 已就绪，但没有任何路由
//   - httpForwarder/httpsForwarder 为 nil（未设置，后续请求会 fallback 到关闭连接）
//   - httpsTLSConfig 为 nil（未设置 TLS 证书，HTTPS 连接会被 brokenTLSRouter 关闭）
func NewRouter(log *zap.Logger, providerPrecedence []string) (*Router, error) {
	// ── 创建三层 Muxer ──
	// tcpmuxer.NewMuxer 返回 *Muxer：
	//   - 内部维护路由树，支持 HostSNI 匹配
	//   - providerPrecedence 决定了规则冲突时的胜利者
	//
	// if err != nil：muxer 创建失败
	//   可能原因：providerPrecedence 为空（没有可用的 provider）
	//   处理：立即返回 nil 和错误，不创建 Router（因为没有 muxer 的 router 是无意义的）
	muxTCP, err := tcpmuxer.NewMuxer(providerPrecedence)
	if err != nil {
		return nil, err
	}

	// 第二次创建 muxerTCPTLS，使用相同的 providerPrecedence 列表
	// 注意：确保 providerPrecedence 的语义在三层 muxer 中保持一致
	muxTCPTLS, err := tcpmuxer.NewMuxer(providerPrecedence)
	if err != nil {
		return nil, err
	}

	// 第三次创建 muxerHTTPS
	muxHTTPS, err := tcpmuxer.NewMuxer(providerPrecedence)
	if err != nil {
		return nil, err
	}

	return &Router{
		log:         log,
		muxerTCP:    *muxTCP, // 解引用指针，存储 Muxer 值（非指针），避免外部修改
		muxerTCPTLS: *muxTCPTLS,
		muxerHTTPS:  *muxHTTPS,
	}, nil
}

// HTTP3TLSConfigMatcherFunc 返回一个用于 HTTP/3 的 TLS 配置匹配函数。
//
// HTTP/3 (QUIC) 在传输层使用 UDP，但仍然需要 TLS 证书进行连接建立。
// QUIC 的 TLS 握手与 TCP 的不同，它发生在 UDP 数据包中。
// 因此需要提供一个独立的 TLS 配置匹配器，根据客户端请求的 SNI 返回正确的 TLS 配置。
//
// 返回的闭包函数接受 ConnData，返回 (TLS配置, 配置名称, 错误)：
//  1. 用 muxerHTTPS.Match 匹配 SNI → 找到对应的 TLSHandler
//  2. 从 TLSHandler 中提取 TLS 配置和配置名称
//  3. 如果未匹配到路由，返回默认 TLS 配置
//
// 使用示例（在 HTTP/3 服务器中）：
//
//	matcher := router.HTTP3TLSConfigMatcherFunc()
//	quicServer := quic.Server{
//	    TLSConfig: &tls.Config{
//	        GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
//	            connData := tcpmuxer.ConnData{
//	                ServerName: info.ServerName,
//	            }
//	            cfg, name, err := matcher(connData)
//	            return cfg, err
//	        },
//	    },
//	}
//
// HTTP/2 通过 TCP 不需要此函数，因为 tls.Server 直接使用
// Router 的 hostHTTPTLSConfig 和 httpsTLSConfig。
func (r *Router) HTTP3TLSConfigMatcherFunc() func(connData tcpmuxer.ConnData) (*tls.Config, string, error) {
	return func(connData tcpmuxer.ConnData) (*tls.Config, string, error) {
		// 仅在 muxerHTTPS 中匹配（HTTP/3 使用 HTTPS 路由的 TLS 配置）
		// 忽略 catchAll 布尔值（_）：HTTP/3 中即使匹配的是 catchall 也使用该配置
		h, _ := r.muxerHTTPS.Match(connData)

		// if h == nil 为 true：
		//   SNI 在 muxerHTTPS 中没有匹配的路由 → 返回默认 TLS 配置
		//   h == nil 的典型场景：
		//     - 从未调用 SetHTTPSForwarder（此时 httpsTLSConfig 也为 nil）
		//     - SNI 不匹配任何已注册的 HostSNI 规则
		if h == nil {
			return r.httpsTLSConfig, nmqtls.DefaultTLSConfigName, nil
		}

		// ── 类型断言：检查 handler 是否是 TLSHandler ──
		// muxerHTTPS 中的 handler 可能是：
		//   - *tcp.TLSHandler：通过 SetHTTPSForwarder 添加的正常 HTTPS 路由
		//   - *brokenTLSRouter：TLS 配置损坏时添加的占位符
		//
		// if tlsHandler, ok := h.(*tcp.TLSHandler); ok 为 true：
		//   handler 是正常的 TLSHandler → 提取其中的 TLS 配置和选项名称
		//
		// if ok 为 false（类型断言失败）：
		//   handler 不是 TLSHandler（可能是 brokenTLSRouter 或其他类型）
		//   → 跳过此分支，返回错误
		if tlsHandler, ok := h.(*tcp.TLSHandler); ok {
			return tlsHandler.Config, tlsHandler.TLSOptionsName, nil
		}

		// handler 存在但类型不对 → 配置错误，返回 nil, "", 错误
		return nil, "", errors.New("handler is not a tls handler")
	}
}

// ServeTCP 将 TCP 连接分发到正确的 TCP/HTTP handler。
//
// 这是 Router 的入口方法，每一条新 TCP 连接都会调用 ServeTCP。
// 它的核心职责是：对连接进行协议检测（Postgres? TLS? HTTP?），
// 然后根据路由规则找到最匹配的 handler 来处理该连接。
//
// # 完整处理流程
//
//	新连接进来
//	  │
//	  ├── 第1步：快速路径——仅纯 TCP 路由且无 TLS/HTTPS 路由时，直接匹配 TCP muxer
//	  │
//	  ├── 第2步：创建 peekConn，Peek 首字节进行协议检测
//	  │        ├── isPostgres? → servePostgres → 返回
//	  │        └── 非 Postgres → 继续
//	  │
//	  ├── 第3步：读取 TLS ClientHello（peek，不消费字节）
//	  │        ├── 读取出错 → 关闭连接，返回
//	  │        └── 成功读取 → 去除 deadline，继续
//	  │
//	  ├── 第4步：构建 ConnData（SNI + ALPN 协议列表）
//	  │
//	  ├── 第5步：非 TLS 连接 → muxerTCP 匹配 → httpForwarder fallback
//	  │
//	  ├── 第6步：ACME-TLS/1 直通检查
//	  │
//	  ├── 第7步：HTTPS 路由匹配（精确匹配优先）
//	  │        ├── handlerHTTPS != nil && !catchAll → 使用 HTTPS 路由
//	  │        └── 否则继续
//	  │
//	  ├── 第8步：TCP-TLS 路由匹配（精确匹配优先）
//	  │        ├── handlerTCPTLS != nil && !catchAll → 使用 TCP-TLS 路由
//	  │        └── 否则继续
//	  │
//	  ├── 第9步：HTTPS catchall fallback
//	  │        ├── handlerHTTPS != nil → 使用 HTTPS catchall
//	  │        └── 否则继续
//	  │
//	  ├── 第10步：TCP-TLS catchall fallback
//	  │        ├── handlerTCPTLS != nil → 使用 TCP-TLS catchall
//	  │        └── 否则继续
//	  │
//	  └── 第11步：最后 fallback
//	           ├── httpsForwarder != nil → 使用默认 HTTPS 处理
//	           └── 否则关闭连接
//
// 使用示例：
//
//	// Router 在 TCP 服务器中作为连接处理器：
//	srv := tcp.NewServer(addr)
//	srv.OnConnection(func(conn tcp.WriteCloser) {
//	    router.ServeTCP(conn) // 每条新连接都会调用此方法
//	})
//
//	// 每个 ServeTCP 调用都是阻塞的（处理完连接才返回），
//	// TCP 服务器应该为每个连接启动一个 goroutine 调用 ServeTCP。
func (r *Router) ServeTCP(conn tcp.WriteCloser) {
	// ========================================================================
	// 第1步：快速路径 —— 纯 TCP 场景提前分流
	// ========================================================================
	// 如果入口只有纯 TCP 路由（没有 TLS 路由也没有 HTTPS 路由），
	// 并且至少有一条 TCP 路由，可以直接用 muxerTCP 匹配。
	//
	// 为什么要做这个快速路径？
	//   纯 TCP 客户端（如 MySQL、Redis 客户端）不会主动发送数据，
	//   而是等待服务端发送握手包。如果走下面的 clientHelloInfo 路径，
	//   会尝试 Peek 连接，因为客户端没有发数据，Peek 会永久阻塞。
	//   因此需要先检测"只有纯 TCP 路由"的场景，跳过协议检测。
	//
	// 条件拆解：
	//
	//   r.muxerTCP.HasRoutes() 为 true：
	//     至少有一条 TCP 路由已注册。如果 TCP 路由为空，不需要走快速路径。
	//
	//   && !r.muxerTCPTLS.HasRoutes() 为 true：
	//     没有 TCP-TLS 路由。如果有 TCP-TLS 路由，连接可能是 TLS 的，
	//     就不能在快速路径中处理（需要读取 ClientHello 判断是否真的是 TLS）。
	//
	//   && !r.muxerHTTPS.HasRoutes() 为 true：
	//     没有 HTTPS 路由。如果有 HTTPS 路由，连接可能是 HTTPS 的，同样不能提前处理。
	//
	//   三个条件同时为 true：入口只有纯 TCP 路由 → 安全地使用快速路径
	//
	//   如果任一条件为 false（有 TLS 或 HTTPS 路由）：
	//     跳过快速路径，进入完整的协议检测流程
	//
	//   如果三个条件都满足但没有匹配的路由：
	//     handler 为 nil，不提前返回，继续往下走 → 最终关闭连接（因为 fallback 也没有 handler）
	if r.muxerTCP.HasRoutes() && !r.muxerTCPTLS.HasRoutes() && !r.muxerHTTPS.HasRoutes() {
		// 构建 ConnData，SNI 为空字符串（纯 TCP 没有 SNI）
		connData, err := tcpmuxer.NewConnData("", conn.RemoteAddr(), nil)
		if err != nil {
			// if err != nil：构建 ConnData 失败（通常是 RemoteAddr 格式异常）
			//   → 记录错误，关闭连接，return（连接无法处理）
			r.log.Error("Error while reading TCP connection data", zap.Error(err))
			_ = conn.Close()
			return
		}

		// 进行路由匹配，匹配上之后将选中的路由 handler 返回
		handler, _ := r.muxerTCP.Match(connData)

		// if handler != nil 为 true：
		//   ConnData 匹配到了 muxerTCP 中的某条路由规则
		//   → 用匹配到的 handler 处理连接
		//
		// if handler != nil 为 false：
		//   所有已注册的 TCP 路由都不匹配此连接（如 SNI 不匹配）
		//   → 不提前返回，继续往下走：
		//       1. 可能实际上是 HTTP 请求（虽然我们没有 HTTPS 路由但可能有 HTTP）
		//       2. 可能是 HTTPS 请求，即使没有 TLS 路由，也需要返回 404
		//          （因为客户端期待 TLS 连接，必须给一个响应）
		if handler != nil {
			// Remove read/write deadline and delegate this th underlying TCP server
			// 移除读写超时：此后连接的超时控制交给底层的 TCP 服务器
			// time.Time{} 是零值，表示"没有 deadline"
			//
			// if err = conn.SetDeadline(time.Time{}); err != nil：
			//   设置 deadline 失败（极少见，可能是连接已关闭）
			//   → 记录错误但不退出，继续用 handler 处理（尽力而为）
			if err = conn.SetDeadline(time.Time{}); err != nil {
				r.log.Error("Error while setting deadline")
			}

			// 使用 handler 处理 TCP 连接
			handler.ServeTCP(conn)
			return
		}
		// Otherwise we keep going because:
		// 1. we could be in the case where we have HTTP routers
		// 2. if it is an HTTPS request, even though we do not have any TLS routers
		// we still need to reply with a 404 because the client is expecting a TLS connection
	}

	// ========================================================================
	// 第2步：创建 peekConn + 协议检测（Postgres）
	// ========================================================================
	// TODO check if ProxyProtocol changes the first bytes of the request

	// newPeekConn 包装原始连接，提供 Peek（偷看但不消费字节）能力。
	// 底层使用 bufio.Reader 进行缓冲读取：
	//   - Peek(n)：偷看前 n 个字节，但不从缓冲区移除（用于协议检测）
	//   - Read(p)：先消费 peeked 缓冲区，再从 bufio.Reader 读取（用于后续正常 IO）
	pConn := newPeekConn(conn)

	// ── Postgres 协议检测 ──
	// isPostgres 通过 Peek 连接的首字节来判断是否为 Postgres 协议：
	//   - Postgres 的 StartupMessage 以 int32 长度开头，首字节通常为 0x00
	//   - SSLRequest 的首 4 字节为 int32(8)，其中首字节为 0x00
	//   - 检测逻辑详见 isPostgres 函数
	//
	// postgres 为 true：连接使用的是 Postgres 协议
	// postgres 为 false：不是 Postgres，继续后续协议检测
	//
	// if err != nil：Peek 首字节失败
	//   → 进入错误处理分支
	postgres, err := isPostgres(pConn)
	if err != nil {
		// ── 错误分类处理 ──
		// errors.AsType[*net.OpError](err)：尝试将 err 断言为 *net.OpError
		//   net.OpError 表示网络操作层面的错误（超时、连接重置等）
		//   ok=true：err 是网络操作错误 → 可以进一步判断是否是超时
		//   ok=false：err 不是网络操作错误 → 直接进入 errors.Is 判断
		//
		// 联合条件：errors.Is(err, io.EOF) && (!ok || !opErr.Timeout())
		//   分解：
		//     errors.Is(err, io.EOF)：错误链中包含 io.EOF（对端关闭了连接）
		//     && (!ok || !opErr.Timeout())：
		//       !ok：         err 不是 net.OpError → 直接满足（普通 EOF，需要日志）
		//       !opErr.Timeout()：err 是 net.OpError 但不是超时错误（如连接被重置）→ 也需要日志
		//     只有 ok=true && opErr.Timeout()=true（即网络超时导致的 EOF）时才跳过日志
		//
		//   为什么超时导致的 EOF 不记录？
		//     超时是预期的行为（我们设置了较短的初始 deadline 来避免 Peek 永久阻塞），
		//     不需要记录为错误。其他类型的 EOF（如对端主动关闭）才值得记录。
		opErr, ok := errors.AsType[*net.OpError](err)
		if errors.Is(err, io.EOF) && (!ok || !opErr.Timeout()) {
			r.log.Error("Error while reading Postgres connection data", zap.Error(err))
		}
		_ = pConn.Close()
		return
	}

	// 如果是 Postgres 协议，则按照 Postgres 协议进行处理
	// if postgres 为 true：连接是 Postgres 协议 → servePostgres 处理
	//   servePostgres 内部会进行 Postgres 路由匹配和代理转发
	//
	// if postgres 为 false：不是 Postgres → 继续后面的 TLS/HTTP 检测
	if postgres {
		// servePostgres 执行 Postgres 协议的路由匹配和代理
		if err = r.servePostgres(pConn); err != nil {
			// ── 同样的错误分类逻辑 ──
			// 静默非超时的非 EOF 错误（Postgres 协议处理过程中的预期错误）
			// 只记录非预期的错误
			opErr, ok := errors.AsType[*net.OpError](err)
			if !errors.Is(err, io.EOF) && (!ok || !opErr.Timeout()) {
				// 条件：err 不是 EOF，且（err 不是网络错误，或不是超时错误）
				//   → 这是一个非预期的、值得记录的错误
				r.log.Error("Error while reading Postgres connection data", zap.Error(err))
			}
		}
		_ = pConn.Close()
		return
	}

	// ========================================================================
	// 第3步：读取 TLS ClientHello（peek，不消费字节）
	// ========================================================================
	// clientHelloInfo 通过假的 TLS 握手读取 ClientHello 内容：
	//   - peek 连接的首字节判断是否是 TLS 记录
	//   - 如果是 TLS，用 GetConfigForClient 回调捕获 SNI 和 ALPN
	//   - 使用 errClientHelloRead 中断握手，不实际完成 TLS 握手
	//   - 返回的 clientHello 包含：serverName(SNI)、protos(ALPN)、isTLS(是否 TLS)
	hello, err := clientHelloInfo(pConn)
	if err != nil {
		// ── 读取 ClientHello 失败 ──
		// err != nil：无法读取或解析 ClientHello
		//   可能原因：非 TLS 连接且首字节不是 0x16、连接已断开、Peek 出错
		//
		// 同样的错误分类：只记录非超时、非 EOF 的错误
		opErr, ok := errors.AsType[*net.OpError](err)
		if !errors.Is(err, io.EOF) && (!ok || !opErr.Timeout()) {
			// !errors.Is(err, io.EOF)：err 不是 EOF（对端正常关闭不记录）
			// && (!ok || !opErr.Timeout())：不是网络超时错误（超时不记录）
			// → 这两类之外的错误才值得记录
			r.log.Error("Error while reading TLS ClientHello", zap.Error(err))
		}
		_ = pConn.Close()
		return
	}

	// The deadline was set to avoid blocking on the initial read of the ClientHello.
	// but now we have it, we can remove it,
	// and delegate this to underlying TCP server (for now only handled by HTTP server)
	// 此时我们已经成功读取了 ClientHello，不再需要防止阻塞的 deadline。
	// 移除 deadline，将超时控制下放给底层 handler（如 HTTP 服务器）。
	//
	// if err = pConn.SetDeadline(time.Time{}); err != nil：
	//   time.Time{} 是零值 → 表示移除所有 deadline
	//   如果失败（极少见），记录错误但不中断处理，因为连接本身还活着
	if err = pConn.SetDeadline(time.Time{}); err != nil {
		r.log.Error("Error while setting deadline")
	}

	// ========================================================================
	// 第4步：构建 ConnData
	// ========================================================================
	// ConnData 包含路由匹配所需的所有连接元数据：
	//   - hello.serverName：TLS SNI（如 "example.com"），用于 HostSNI 规则匹配
	//   - pConn.RemoteAddr()：客户端地址，用于 ClientAddr 规则匹配
	//   - hello.protos：ALPN 协议列表（如 ["h2", "http/1.1"]）
	connData, err := tcpmuxer.NewConnData(hello.serverName, pConn.RemoteAddr(), hello.protos)
	if err != nil {
		// if err != nil：构建 ConnData 失败（元数据异常）
		//   → 记录错误，关闭连接
		r.log.Error("Error while reading TCP connection data", zap.Error(err))
		_ = pConn.Close()
		return
	}

	// ========================================================================
	// 第5步：非 TLS 连接的 TCP 路由匹配
	// ========================================================================
	// 如果不是 TLS 数据（首字节不是 0x16 也不是 SSLv2）
	//
	// if !hello.isTLS 为 true：连接是明文 HTTP 或纯 TCP
	//   → 走 muxerTCP 路由匹配，fallback 到 httpForwarder
	//
	// if !hello.isTLS 为 false：连接使用了 TLS
	//   → 跳过此分支，进入后续的 TLS 相关路由匹配
	if !hello.isTLS {
		// 匹配 TCP 路由，匹配上之后将选中的路由 handler 返回
		handler, _ := r.muxerTCP.Match(connData)

		// switch 三路分发（不使用 if-else 链，更清晰）：
		switch {
		case handler != nil:
			// case 1：匹配到了 TCP 路由
			//   → 用匹配到的 handler 处理连接
			//   例如：HostSNI(`example.com`) 匹配纯 TCP 代理路由
			handler.ServeTCP(pConn)

		case r.httpForwarder != nil:
			// case 2：没有匹配的 TCP 路由，但有 HTTP 转发器
			//   → 连接是明文 HTTP 请求，交给 httpForwarder 解析 HTTP 并转发
			//   典型场景：HTTP/1.1 明文请求到达路由器
			r.httpForwarder.ServeTCP(pConn)

		default:
			// case 3：既没有 TCP 路由，也没有 HTTP 转发器
			//   → 没有人能处理此连接，关闭它
			_ = pConn.Close()
			return
		}
		// 注意：此处没有 return！如果是非 TLS 连接但下面有 ACME 检查，
		// 逻辑上不应该到达（因为 isTLS=false，但 ACME-TLS/1 是 TLS 协议）。
		// 然而代码流程上会继续执行到第6步，所以需要看第6步的条件：
		//   !r.acmeTLSPassthrough && slices.Contains(hello.protos, ...)
		// 非 TLS 情况下 hello.protos 为空切片 → 条件不成立，安全。
		// 但这里实际上存在一个隐藏的控制流问题：
		//   如果 switch 中的 case handler != nil 或 case httpForwarder != nil 匹配，
		//   ServeTCP 可能处理完成后返回（取决于 handler 的实现），
		//   但如果没有显式 return，代码会继续执行。
		//   实际上 ServeTCP(pConn) 完成后，函数继续往下走会走到 HTTPS/TLS 路由，
		//   这时 pConn 已经被消费了，可能导致二次处理或数据竞争。
		// 这是一个已知的设计缺陷（代码中没有 return，依赖 handler.ServeTCP 内部行为）。
	}

	// ========================================================================
	// 第6步：ACME-TLS/1 挑战处理
	// ========================================================================
	// ACME (Automatic Certificate Management Environment) TLS-ALPN-01 验证：
	//   Let's Encrypt 等 CA 使用此协议验证域名所有权，颁发 TLS 证书。
	//   客户端在 TLS 握手中携带 "acme-tls/1" ALPN 协议，
	//   服务端需要用特殊方式完成握手以通过验证。
	//
	// 条件：!r.acmeTLSPassthrough && slices.Contains(hello.protos, tlsalpn01.ACMETLS1Protocol)
	//
	//   !r.acmeTLSPassthrough 为 true：
	//     ACME TLS 直通未启用 → 由 Router 自己处理 ACME 验证
	//     如果 acmeTLSPassthrough=true，ACME 流量应该直通到后端服务处理
	//
	//   && slices.Contains(hello.protos, tlsalpn01.ACMETLS1Protocol) 为 true：
	//     客户端 ALPN 协议列表中包含 "acme-tls/1"
	//
	//   两者同时为 true → 这是一个 ACME 验证请求，Router 自己处理
	//
	// 如果 !r.acmeTLSPassthrough 为 false（acmeTLSPassthrough=true）：
	//   ACME 流量直通 → 不拦截，继续后续路由匹配
	//
	// 如果 Contains 为 false（请求不包含 acme-tls/1 ALPN）：
	//   普通 TLS 流量 → 继续后续路由匹配
	if !r.acmeTLSPassthrough && slices.Contains(hello.protos, tlsalpn01.ACMETLS1Protocol) {
		r.acmeTLSALPNHandler().ServeTCP(pConn)
		return
	}

	// ========================================================================
	// 第7步：HTTPS 路由匹配（精确匹配优先）
	// ========================================================================
	// Match 返回两个值：
	//   handlerHTTPS：匹配到的 handler（可能为 nil = 无匹配）
	//   catchAllHTTPS：true 表示匹配到的是 HostSNI(*)（通配符）规则
	//
	// handlerHTTPS != nil && !catchAllHTTPS 为 true：
	//   匹配到了非通配符的 HTTPS 路由（如 HostSNI(`example.com`)）
	//   → 优先使用，直接处理。此时使用的是该 SNI 特定的 TLS 配置。
	//
	// handlerHTTPS != nil && !catchAllHTTPS 为 false（两者之一不满足）：
	//   - handlerHTTPS == nil：HTTPS 路由中没有匹配项
	//   - catchAllHTTPS == true：匹配到 HostSNI(*) 通配符（低优先级）
	//   这两种情况都不优先 → 让 TCP-TLS 路由有机会匹配
	handlerHTTPS, catchAllHTTPS := r.muxerHTTPS.Match(connData)
	if handlerHTTPS != nil && !catchAllHTTPS {
		// In order not to depart from the behavior in 2.6,
		// we only allow an HTTPS router to take precedence over a TCP-TLS router
		// if it is _not_ an HostSNI(*) router
		// (so basically any router that has a specific HostSNI based rule).
		// 为了保持向后兼容（v2.6 的行为），只有精确的 HostSNI 匹配
		// 才允许 HTTPS 路由优先于 TCP-TLS 路由。
		handlerHTTPS.ServeTCP(pConn)
		return
	}

	// ========================================================================
	// 第8步：TCP-TLS 路由匹配（精确匹配优先）
	// ========================================================================
	// 包含 TCP TLS passthrough 路由。
	// TLS Passthrough 场景：路由器不解密 TLS，原封不动将 TLS 加密流量转发到后端。
	// 常见于非 HTTP 的 TLS 协议（如 TLS 加密的 TCP 服务、gRPC over TLS 等）。
	//
	// if handlerTCPTLS != nil && !catchALLTCPTLS 为 true：
	//   匹配到了非通配符的 TCP-TLS 路由
	//   → 用该路由的 TLS 配置处理连接
	//
	// 为什么 TCP-TLS 排在 HTTPS 之后？
	//   HTTP 服务通常比纯 TCP 服务更常见，且 HTTPS 路由可以解密后进一步做
	//   HTTP 层面的路由匹配（Path、Headers 等），比 TCP-TLS 的粒度更细。
	handlerTCPTLS, catchALLTCPTLS := r.muxerTCPTLS.Match(connData)
	if handlerTCPTLS != nil && !catchALLTCPTLS {
		handlerTCPTLS.ServeTCP(pConn)
		return
	}

	// ========================================================================
	// 第9步：HTTPS catchall fallback
	// ========================================================================
	// 到达这里说明没有精确匹配的 HTTPS 或 TCP-TLS 路由。
	// 尝试使用 HTTPS 的 catchall（HostSNI(*) 规则）。
	//
	// 典型场景：
	//   有一个只有 PathPrefix 规则的 HTTPS 路由，没有配置具体的 HostSNI。
	//   在内部，没有 HostSNI 的规则会被映射为 HostSNI(*)（通配符）。
	//   此时 handlerHTTPS 不为 nil，但 catchAllHTTPS 为 true，
	//   所以第7步和第8步都不成立，走到这里用 HTTPS catchall 处理。
	//
	// if handlerHTTPS != nil 为 true：
	//   存在 HTTPS catchall 路由 → 使用它
	//
	// if handlerHTTPS != nil 为 false：
	//   handlerHTTPS == nil → 没有 HTTPS 路由 → 继续尝试 TCP-TLS catchall
	if handlerHTTPS != nil {
		handlerHTTPS.ServeTCP(pConn)
		return
	}

	// ========================================================================
	// 第10步：TCP-TLS catchall fallback
	// ========================================================================
	// if handlerTCPTLS != nil 为 true：
	//   存在 TCP-TLS catchall 路由 → 使用它
	//
	// if handlerTCPTLS != nil 为 false：
	//   没有任何 TLS 路由（精确或通配符）→ 进入最终 fallback
	if handlerTCPTLS != nil {
		handlerTCPTLS.ServeTCP(pConn)
		return
	}

	// ========================================================================
	// 第11步：最终 fallback —— 默认 HTTPS 转发
	// ========================================================================
	// 这是所有路由都未匹配时的最后兜底。
	// 使用 httpsForwarder（默认 TLS 配置解密后转发到 HTTPS handler）。
	// 这是最常见的情况：所有 HTTPS 请求最终都走到这里，
	// 使用默认证书解密，然后通过 HTTPS router 做 HTTP 层面的路由匹配。
	//
	// if r.httpsForwarder != nil 为 true：
	//   有默认的 HTTPS 转发器 → 使用默认证书解密并转发
	//
	// if r.httpsForwarder != nil 为 false：
	//   没有任何 handler 可以处理 → 关闭连接（最后的手段）
	if r.httpsForwarder != nil {
		r.httpsForwarder.ServeTCP(pConn)
		return
	}

	// 没有任何 handler 可用，关闭连接
	_ = pConn.Close()
	return
}

// AddTCPRoute 为给定的规则定义一个 TCP handler。
//
// 将路由规则注册到 muxerTCP 中，当连接的元数据匹配该规则时，
// muxerTCP.Match 会返回此 target handler。
//
// 参数说明：
//
//	rule:         路由规则（如 "HostSNI(`example.com`)"）
//	              muxer 内部会解析此字符串，提取匹配条件
//	priority:     规则优先级（数值越大优先级越高），用于同 provider 内排序
//	providerName: provider 名称（如 "file"、"kubernetes"），
//	              用于跨 provider 的优先级比较
//	target:       匹配成功后处理连接的 handler
//
// 使用示例：
//
//	// 注册一条 TCP 路由：所有 SNI 为 example.com 的 TCP 连接
//	myHandler := tcp.HandlerFunc(func(conn tcp.WriteCloser) {
//	    // 处理 TCP 连接...
//	})
//	err := router.AddTCPRoute("HostSNI(`example.com`)", 10, "file", myHandler)
//	if err != nil {
//	    // 路由添加失败（规则解析错误或冲突）
//	}
//
// 注意：
//   - AddTCPRoute 只注册到 muxerTCP，不涉及 TLS
//   - 规则的第二个参数传空字符串（已被 rule 参数隐含）
func (r *Router) AddTCPRoute(rule string, priority int, providerName string, target tcp.Handler) error {
	return r.muxerTCP.AddRoute(rule, "", priority, providerName, target)
}

// AddHTTPTLSConfig 注册一个 SNI 主机及其对应的 TLS 配置。
//
// 此方法不会立即创建路由，只是将 SNI → TLS 配置的映射记录下来。
// 真正的路由注册发生在 SetHTTPSForwarder 中（遍历 hostHTTPTLSConfig 时）。
// 这样设计的原因是：HTTPS 的 handler 需要知道具体的 TLS 配置，
// 而 TLS 配置在 SetHTTPSForwarder 调用时才最终确定。
//
// 参数说明：
//
//	sniHost:     SNI 主机名（如 "example.com", "*.example.com"）
//	config:      TLS 配置（证书、曲线、加密套件等）
//	             nil 表示配置不可用，后续将创建 brokenTLSRouter 直接关闭连接
//	optionsName: TLSOption 名称（如 "default", "my-cert"），用于日志和标识
//
// 使用示例：
//
//	// 为 example.com 注册 TLS 证书
//	tlsCfg := &tls.Config{
//	    Certificates: []tls.Certificate{myCert},
//	}
//	router.AddHTTPTLSConfig("example.com", tlsCfg, "example-com-cert")
//
//	// 为没有证书的域名注册 nil，连接时直接关闭
//	router.AddHTTPTLSConfig("no-cert.example.com", nil, "no-cert")
//
//	if r.hostHTTPTLSConfig == nil 为 true：
//	   hostHTTPTLSConfig map 尚未初始化（第一次调用此方法）
//	   → 先创建空 map，再存储 SNI 配置
//
//	if r.hostHTTPTLSConfig == nil 为 false：
//	   map 已存在 → 直接存储（如果 SNI 重复，后添加的覆盖前者）
func (r *Router) AddHTTPTLSConfig(sniHost string, config *tls.Config, optionsName string) {
	if r.hostHTTPTLSConfig == nil {
		r.hostHTTPTLSConfig = map[string]tlsConfigWithOptionsName{}
	}

	r.hostHTTPTLSConfig[sniHost] = tlsConfigWithOptionsName{
		cfg:         config,
		optionsName: optionsName,
	}
}

// GetHTTPHandler 返回当前持有的 HTTP handler。
// 注意：此方法返回的是 httpsHandler（而非 httpHandler），这可能是命名错误或历史遗留问题。
func (r *Router) GetHTTPHandler() http.Handler {
	return r.httpsHandler
}

// GetHTTPSHandler 返回当前持有的 HTTPS handler。
func (r *Router) GetHTTPSHandler() http.Handler {
	return r.httpsHandler
}

// SetHTTPForwarder 设置处理 HTTP 明文请求的 TCP 转发器。
//
// 当 muxerTCP 未匹配到路由且连接是明文 HTTP 时（ServeTCP 第5步），
// 使用此 forwarder 处理连接。通常实现为 HTTP 服务器，解析 HTTP 请求头
// 并转发到后端。
//
// 使用示例：
//
//	// 创建一个 HTTP 到后端代理的 handler
//	httpProxy := createHTTPProxyHandler()
//	router.SetHTTPForwarder(httpProxy)
func (r *Router) SetHTTPForwarder(handler tcp.Handler) {
	r.httpForwarder = handler
}

// SetHTTPSForwarder 设置处理 HTTPS 请求的 TCP 转发器，并为每个已注册的
// Host(SNI) 规则创建对应的 TLS 路由。
//
// 这是 HTTPS 路由体系的核心初始化方法，完成了两个关键步骤：
//
//  1. 为每个通过 AddHTTPTLSConfig 注册的 SNI 主机创建路由规则：
//     - 如果 TLS 配置不为 nil → 创建 TLSHandler（解密后交给传入的 handler）
//     - 如果 TLS 配置为 nil → 创建 brokenTLSRouter（直接关闭连接，因为没有可用证书）
//     - 路由规则格式：HostSNI(`sniHost`)，优先级使用 tcpmuxer.GetRulePriority 自动计算
//
//  2. 设置 httpsForwarder（默认 HTPS 转发器）：
//     - 如果 httpsTLSConfig 不为 nil → 创建 TLSHandler 使用默认证书
//     - 如果 httpsTLSConfig 为 nil → 创建 brokenTLSRouter（所有 HTTPS 连接将被关闭）
//
// 参数 handler：解密后的 HTTP 请求的实际处理器（通常是 HTTP router）
//
// 使用示例：
//
//	// 配置 HTTPS 转发器
//	router.SetHTTPTLSConfig(httpsTLSConfig) // 先设置默认 TLS 配置
//	router.AddHTTPTLSConfig("example.com", exampleTLSCfg, "example-cert")
//	router.AddHTTPTLSConfig("api.example.com", apiTLSCfg, "api-cert")
//
//	httpsHandler := createHTTPSProxyHandler() // 创建 HTTPS → 后端的代理
//	router.SetHTTPSForwarder(httpsHandler)
//	// 此时：
//	//   - muxerHTTPS 中有 HostSNI(`example.com`) 和 HostSNI(`api.example.com`) 的路由
//	//   - httpsForwarder 使用默认证书处理其他 SNI 的请求
func (r *Router) SetHTTPSForwarder(handler tcp.Handler) {
	// ── 第1步：为每个已注册的 SNI 创建 HTTPS 路由 ──
	// 遍历 hostHTTPTLSConfig map：
	//   key=sniHost（如 "example.com"），value=tlsConf（TLS 配置 + 选项名称）
	for sniHost, tlsConf := range r.hostHTTPTLSConfig {
		var tcpHandler tcp.Handler

		// if tlsConf.cfg == nil 为 true：
		//   该 SNI 的 TLS 配置不可用（可能证书加载失败）
		//   → 使用 brokenTLSRouter：任何访问此域名的连接都将被直接关闭
		//
		// if tlsConf.cfg == nil 为 false：
		//   TLS 配置正常 → 创建 TLSHandler：
		//     - Next:           传入的 handler（TLS 解密后的流量交给它）
		//     - Config:         该 SNI 专用的 TLS 配置（证书等）
		//     - TLSOptionsName: 配置来源名称（用于日志追踪）
		if tlsConf.cfg == nil {
			tcpHandler = &brokenTLSRouter{}
		} else {
			tcpHandler = &tcp.TLSHandler{
				Next:           handler,
				Config:         tlsConf.cfg,
				TLSOptionsName: tlsConf.optionsName,
			}
		}

		// 构建路由规则：HostSNI(`sniHost`)
		// 例如：sniHost="example.com" → rule="HostSNI(`example.com`)"
		rule := fmt.Sprintf("HostSNI(`%s`)", sniHost)

		// 将路由添加到 muxerHTTPS：
		//   - muxerHTTPS 是 HTTPS 路由表，专门用于 TLS 解密后的路由匹配
		//   - providerName 传空字符串：因为每个 SNI 只有一个 TLS 配置，
		//     不存在跨 provider 的冲突，不需要 tie-breaking
		//   - 优先级由 GetRulePriority(rule) 自动计算
		//
		// if err := ... ; err != nil 为 true：
		//   添加路由失败（规则解析错误等）→ 记录错误但继续处理其他 SNI
		//   不影响其他正常的 SNI 配置
		if err := r.muxerHTTPS.AddRoute(rule, "", tcpmuxer.GetRulePriority(rule), "", tcpHandler); err != nil {
			r.log.Error("Error while adding route", zap.Error(err))
		}
	}

	// ── 第2步：设置默认 HTTPS 转发器 ──
	// if r.httpsTLSConfig == nil 为 true：
	//   没有默认的 TLS 配置（未设置默认证书）
	//   → 使用 brokenTLSRouter 直接关闭连接
	//   这意味着：只处理精确匹配 SNI 的 HTTPS 请求，其他 SNI 的请求会被关闭
	//
	// if r.httpsTLSConfig == nil 为 false：
	//   有默认 TLS 配置 → 创建 TLSHandler 使用默认证书
	//   这确保了任何 SNI 的 HTTPS 请求至少有一个 fallback
	if r.httpsTLSConfig == nil {
		r.httpsForwarder = &brokenTLSRouter{}
		return
	}

	r.httpsForwarder = &tcp.TLSHandler{
		Next:           handler,
		Config:         r.httpsTLSConfig,
		TLSOptionsName: nmqtls.DefaultTLSConfigName,
	}

}

// SetHTTPHandler 设置 HTTP handler，供 Switcher 在配置重新加载时使用。
//
// 此 handler 不直接在 ServeTCP 中使用（ServeTCP 使用 httpForwarder），
// 而是在配置重新加载时由 Switcher 注入到最新的 handler 中，
// 实现旧连接用旧 handler、新连接用新 handler 的平滑过渡。
func (r *Router) SetHTTPHandler(handler http.Handler) {
	r.httpHandler = handler
}

// SetHTTPSHandler attaches https handlers on the router.
func (r *Router) SetHTTPSHandler(handler http.Handler, config *tls.Config) {
	r.httpsHandler = handler
	r.httpsTLSConfig = config
}

// EnableACMETLSPassthrough 启用或禁用 ACME-TLS/1 挑战的直通模式。
//
// 如果 enable=true：ACME-TLS/1 流量不经过 Router 的 acmeTLSALPNHandler，
//
//	而是直接走后续路由匹配，由下游服务处理 ACME 验证。
//	适用于后端服务自己管理 ACME 证书的场景。
//
// 如果 enable=false（默认）：ACME-TLS/1 流量由 Router 的 acmeTLSALPNHandler
//
//	拦截处理，完成域名所有权验证。
//	适用于 Router 统一管理 ACME 证书的场景。
func (r *Router) EnableACMETLSPassthrough(enable bool) {
	r.acmeTLSPassthrough = enable
}

// acmeTLSALPNHandler 返回一个处理 ACME-TLS/1 ALPN 挑战的特殊 handler。
//
// ACME (Let's Encrypt) 使用 TLS-ALPN-01 验证域名所有权：
//   - CA 的验证服务器连接到 domain:443
//   - TLS 握手中携带 "acme-tls/1" ALPN 协议
//   - 服务端必须使用正确的 ACME 证书完成 TLS 握手
//   - 握手成功后 CA 即可验证域名所有权，颁发证书
//
// 返回值：
//
//	if r.httpsTLSConfig == nil 为 true：
//	   没有 TLS 配置 → 返回 brokenTLSRouter（关闭连接，无法完成 ACME 验证）
//
//	if r.httpsTLSConfig == nil 为 false：
//	   有 TLS 配置 → 返回正常 handler
func (r *Router) acmeTLSALPNHandler() tcp.Handler {
	// if r.httpsTLSConfig == nil 为 true：
	//   没有默认 TLS 配置（未加载 ACME 证书）
	//   → 无法完成 ACME 验证，返回 brokenTLSRouter 直接关闭连接
	if r.httpsTLSConfig == nil {
		return &brokenTLSRouter{}
	}

	// 返回匿名 handler 函数
	return tcp.HandlerFunc(func(conn tcp.WriteCloser) {
		// 使用 httpsTLSConfig 创建 TLS 服务端连接
		tlsConn := tls.Server(conn, r.httpsTLSConfig)
		defer tlsConn.Close()

		// 设置 2 秒超时：ACME 验证请求预计在很短时间内完成
		// context.Background() 而非使用外部 ctx：
		//   acmeTLSALPNHandler 没有外部 context 传入，需要独立的生命周期
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// 执行 TLS 握手（ACME 验证的核心步骤）
		// if err := tlsConn.HandshakeContext(ctx); err != nil 为 true：
		//   握手失败 → 记录错误（可能是证书问题、网络问题等）
		//   无论成功与否，defer 的 tlsConn.Close() 会释放连接
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			r.log.Error("Error during ACME-TLS/1 handshake", zap.Error(err))
		}
	})
}

// brokenTLSRouter 与一个 TLS 配置损坏/不可用的 Host(SNI) 路由关联。
//
// 作用：确保任何尝试连接到此域名的请求立即被关闭，
// 因为我们无法使用预期的 TLS 配置来处理它。
//
// 何时使用 brokenTLSRouter：
//   - TLS 证书加载失败 → 对应的 SNI 路由被注册为 brokenTLSRouter
//   - 未配置默认 TLS 证书 → httpsForwarder 被设置为 brokenTLSRouter
//   - ACME 直通未就绪 → acmeTLSALPNHandler 返回 brokenTLSRouter
//
// 使用示例（由框架自动创建，用户无需手动使用）：
//
//	// SetHTTPSForwarder 中自动创建：
//	if tlsConf.cfg == nil {
//	    tcpHandler = &brokenTLSRouter{} // SNI 有路由但没有可用证书
//	}
type brokenTLSRouter struct{}

// ServeTCP 立即关闭连接。
// brokenTLSRouter 不做任何实际处理，直接关闭连接以释放资源。
func (t *brokenTLSRouter) ServeTCP(conn tcp.WriteCloser) {
	_ = conn.Close()
}

// peekConn 包装 tcp.WriteCloser，提供 Peek（偷看但不消费字节）能力，
// 以及一个 peeked 缓冲区，用于累积协议检测期间读取的字节以便后续重放。
//
// 设计原理：
//   协议检测需要在连接上"偷看"头几个字节来判断是什么协议（Postgres? TLS? HTTP?），
//   但不能消费这些字节（后续 handler 还需要从头读取完整的协议数据）。
//   bufio.Reader 的 Peek 方法提供了"读但不消费"的能力。
//
//   PeekRead 在读取字节的同时将其缓存到 peeked 切片中，
//   这些字节在后续的 Read 调用中会被优先返回（重放），
//   从而确保后续 handler 看到的仍然是完整的数据流。
//
// 工作流程示例（以检测 Postgres 为例）：
//
//   1. newPeekConn(conn) → 创建 peekConn，内部 reader 为空
//   2. pConn.Peek(1)     → bufio.Reader 从底层连接读取 1 字节到内部缓冲区
//                          返回该字节，但不消费（下次 Peek 还能拿到同样的数据）
//   3. pConn.PeekRead(buf)→ 如果需要读更多字节来确认协议，
//                          从 bufio.Reader 读取并缓存到 peeked 切片
//   4. 确认协议后，handler.ServeTCP(pConn)
//   5. handler 调用 pConn.Read(buf) → 先返回 peeked 中的缓存数据
//                                    → 缓存清空后再从 bufio.Reader 读
//
//	┌─────── peekConn ───────┐
//	│  ┌── peeked []byte ──┐ │   ← 协议检测时缓存的字节（重放区）
//	│  └────────────────────┘ │
//	│  ┌── bufio.Reader ───┐ │   ← 带 Peek 能力的缓冲读取器
//	│  │   (底层 conn)      │ │   ← 原始 TCP 连接
//	│  └────────────────────┘ │
//	└─────────────────────────┘

type peekConn struct {
	tcp.WriteCloser // 嵌入底层 WriteCloser，Write/Close 直接透传

	peeked []byte        // 协议检测过程中累积读取的字节，将在后续 Read 中优先返回
	reader *bufio.Reader // 带 Peek 能力的缓冲读取器（包装底层 conn）
}

// newPeekConn 创建 peekConn，用 bufio.Reader 包装原始连接。
// 默认的 bufio.Reader 缓冲区大小为 4096 字节（bufio 默认值）。
func newPeekConn(conn tcp.WriteCloser) *peekConn {
	return &peekConn{WriteCloser: conn, reader: bufio.NewReader(conn)}
}

// Peek 允许在不消费字节的情况下偷看连接中的前 n 个字节。
//
// 底层使用 bufio.Reader.Peek(n)，它保证：
//   - 返回的切片至少包含 n 个字节（除非遇到错误）
//   - 返回的字节不会被消费，下次 Peek 相同的 n 会返回相同的数据
//   - Peek 后跟 Read，Read 仍能读到 Peek 过的数据
//
// 使用场景：协议检测（判断首字节是 Postgres、TLS Handshake、还是 HTTP）
//
// 使用示例：
//
//	hdr, err := pConn.Peek(1)  // 偷看第1个字节
//	if hdr[0] == 0x16 {        // TLS ClientHello 的首字节
//	    // 这是 TLS 连接
//	}
//	// hdr 的内容还在缓冲区中，后续 Read 仍能读到
func (c *peekConn) Peek(n int) ([]byte, error) {
	return c.reader.Peek(n)
}

// PeekRead 从连接读取数据，同时将读取的字节累积到 peeked 缓冲区中，
// 以便后续 Read 调用时重放。
//
// 注意：PeekRead 会让 reader 前进（消费字节），
// 因此下一次的 Peek 不会返回相同的结果。
//
// if n > 0 为 true：
//
//	reader 成功读取了 n 个字节 → 将 p[:n] 追加到 peeked 缓冲区
//	append 可能触发底层数组扩容（如果 peeked 容量不足）
//
// if n > 0 为 false（n==0）：
//
//	没有读到任何字节（EOF 或临时错误）→ 不追加，直接返回
func (c *peekConn) PeekRead(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.peeked = append(c.peeked, p[:n]...)
	}

	return n, err
}

// Read 先清空 peeked 缓冲区中的累积数据，再从 bufio.Reader 读取。
//
// 这是实现"零字节丢失协议检测"的关键方法：
//  1. 协议检测阶段：用 Peek + PeekRead 读取并缓存字节
//  2. handler 处理阶段：handler 调用 Read → 先返回缓存的字节，再正常读取
//
// if len(c.peeked) > 0 为 true：
//
//	缓冲区中有缓存的字节（协议检测时 PeekRead 累积的）
//	→ 先复制到 p 中（可能只复制部分，如果 p 比 peeked 小）
//	→ 清空已复制的部分
//
//	── 嵌套 if len(c.peeked) == 0 为 true：
//	     peeked 完全清空 → 设为 nil 以释放底层数组（帮助 GC）
//
// if len(c.peeked) > 0 为 false：
//
//	缓冲区已空 → 直接从 bufio.Reader 读取（底层 conn 的数据）
//
// 使用示例：
//
//	// handler 中：
//	buf := make([]byte, 4096)
//	n, err := peekConn.Read(buf) // 先返回之前 PeekRead 缓存的字节
//	// 缓存清空后，后续 Read 正常从连接读取
func (c *peekConn) Read(p []byte) (int, error) {
	if len(c.peeked) > 0 {
		// copy 返回实际复制的字节数（取 p 和 peeked 的最小长度）
		n := copy(p, c.peeked)
		// 截取未复制的部分
		c.peeked = c.peeked[n:]

		// if len(c.peeked) == 0 为 true：
		//   所有缓存的字节都已返回 → 释放底层数组，避免内存泄漏
		//   设置为 nil 而不是空切片：nil slice 的 len==0，但底层数组会被 GC
		if len(c.peeked) == 0 {
			c.peeked = nil
		}
		return n, nil
	}

	return c.reader.Read(p)
}

// clientHello 存储从 TLS ClientHello 消息中提取的关键信息。
//
// 这些信息在 clientHelloInfo 函数中通过假的 TLS 握手提取，
// 用于后续的路由匹配（SNI → HostSNI 规则，ALPN → ACME/HTTP3 判断）。
type clientHello struct {
	serverName string   // TLS SNI（Server Name Indication），如 "example.com"
	protos     []string // ALPN（Application-Layer Protocol Negotiation）协议列表
	// 如 ["h2", "http/1.1"] 或 ["acme-tls/1"]
	isTLS bool // 是否检测到 TLS 协议（首字节为 0x16 或 SSLv2）
}

// clientHelloInfo 从连接中读取 TLS ClientHello 信息，
// 但不消费连接中的任何字节（对后续 handler 透明）。
//
// 实现原理：
//  1. Peek 首字节判断是否是 TLS 记录（0x16 = Handshake）
//  2. 如果是，创建一个假的 tls.Server，其 GetConfigForClient 回调
//     在收到 ClientHello 后立即返回 errClientHelloRead 中断握手
//  3. 回调中捕获 SNI 和 ALPN 信息
//  4. 使用 readOnlyConn 确保假的 TLS 握手不会往连接上写任何数据
//  5. readOnlyConn.Read 使用 PeekRead，读取的字节会被缓存到 peeked 缓冲区
//     因此握手虽然"消费"了数据，但后续 handler 仍能从 peeked 缓冲区读到
//
// 返回值：
//
//	nil, error：Peek 首字节失败（连接已断开等）
//	*clientHello：成功提取的 ClientHello 信息
//	  - isTLS=true：检测到 TLS 连接
//	  - isTLS=false：首字节不是 0x16 也不是 SSLv2
//
// 使用示例：
//
//	hello, err := clientHelloInfo(pConn)
//	if err != nil {
//	    // 读取出错，关闭连接
//	    return
//	}
//	if hello.isTLS {
//	    fmt.Printf("SNI=%s, ALPN=%v\n", hello.serverName, hello.protos)
//	}
func clientHelloInfo(conn *peekConn) (*clientHello, error) {
	// ── 第1步：Peek 首字节判断协议类型 ──
	// Peek(1) 读取第一个字节但不消费
	hdr, err := conn.Peek(1)
	if err != nil {
		// if err != nil：无法从连接 Peek 首字节
		//   可能原因：连接已关闭、对端没有发送任何数据、网络错误
		//   → 返回错误给调用方处理
		return nil, fmt.Errorf("error while peeking first byte: %w", err)
	}

	// No valid TLS record has a byte of 0x80, however SSLv2 handshakes start with
	// an uint16 length where the MSB is set and the first record is always < 256 bytes long.
	// Therefore, typ == 0x80 strongly suggests an SSLv2 client
	const recordTypeSSLv2 = 0x80     // SSLv2 握手特征字节（高位字节）
	const recordTypeHandshake = 0x16 // TLS 握手记录类型

	// if hdr[0] != recordTypeHandshake 为 true：
	//   首字节不是 0x16（TLS Handshake 类型）
	if hdr[0] != recordTypeHandshake {
		// ── 嵌套判断：是否 SSLv2 ──
		// if hdr[0] == recordTypeSSLv2 为 true：
		//   首字节是 0x80 → 很可能是 SSLv2 客户端
		//   SSLv2 是历史协议，标记为 isTLS=true，
		//   后续真正的 TLS 握手会拒绝 SSLv2（因为现代 TLS 不支持 SSLv2）
		//   → 返回 &clientHello{isTLS: true}，让上层走 TLS 路由匹配
		//
		// if hdr[0] == recordTypeSSLv2 为 false：
		//   首字节既不是 0x16 也不是 0x80 → 不是 TLS 连接
		//   → 返回 &clientHello{isTLS: false}，让上层走非 TLS 路由（HTTP/纯 TCP）
		if hdr[0] == recordTypeSSLv2 {
			// we consider SSLv2 as TLS, and it will be refused by real TLS handshake
			return &clientHello{isTLS: true}, nil
		}
		return &clientHello{}, nil
	}

	// ── 第2步：执行假的 TLS 握手以提取 ClientHello 信息 ──
	var (
		sni    string   // 捕获的 SNI 主机名
		protos []string // 捕获的 ALPN 协议列表
	)

	// 创建假的 TLS server：
	//   - readOnlyConn{conn: conn}：只读连接，Write 会失败 → 握手不会实际回复数据
	//   - GetConfigForClient：ClientHello 到达时立即调用此回调
	//     在回调中捕获 SNI 和 ALPN，然后返回 errClientHelloRead 中断握手
	server := tls.Server(readOnlyConn{conn: conn}, &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			// hello.ServerName：客户端 SNI 扩展中的主机名
			sni = hello.ServerName
			// hello.SupportedProtos：客户端 ALPN 扩展中的协议列表
			protos = hello.SupportedProtos
			// 返回 errClientHelloRead 中断握手，不继续 TLS 协商
			return nil, errClientHelloRead
		},
	})

	// ── 第3步：执行握手 ──
	// server.Handshake() 读取客户端的 ClientHello：
	//   1. readOnlyConn.Read → PeekRead → 从 conn 读取 → 缓存到 peeked
	//   2. 解析 ClientHello → 触发 GetConfigForClient → 捕获 SNI/ALPN
	//   3. GetConfigForClient 返回 errClientHelloRead → Handshake 退出
	//
	// if handshakeErr := server.Handshake(); !errors.Is(handshakeErr, errClientHelloRead) 为 true：
	//   错误不是 errClientHelloRead：
	//     - 真正的 TLS 握手错误（协议不兼容、证书问题等）
	//     - 连接读取错误（客户端在握手完成前断开）
	//   → 返回错误给调用方
	//
	// if !errors.Is(handshakeErr, errClientHelloRead) 为 false：
	//   错误就是 errClientHelloRead → 这正是我们期望的！
	//   成功提取了 ClientHello 信息 → 返回结果
	if handshakeErr := server.Handshake(); !errors.Is(handshakeErr, errClientHelloRead) {
		return nil, fmt.Errorf("reading client hello: %w", handshakeErr)
	}

	return &clientHello{serverName: sni, protos: protos, isTLS: true}, nil
}

// readOnlyConn 是一个仅支持读取的 net.Conn 包装器。
//
// 用途：在 clientHelloInfo 的假 TLS 握手中使用，确保：
//   - Read：正常读取（通过 PeekRead 缓存字节到 peeked 缓冲区）
//   - Write：总是失败 → 防止假的 TLS 握手往连接上写任何数据
//   - 其他方法：未实现（如果 TLS 握手意外调用了其他 net.Conn 方法会 panic）
//
// 为什么需要只读连接？
//
//	  clientHelloInfo 只是要"偷看" ClientHello，并不想真正完成 TLS 握手。
//	  但 Go 的 tls.Server 在握手过程中可能会尝试写数据（ServerHello 等）。
//	  通过 readOnlyConn，Write 返回 EOF → 握手无法继续 →
//	  配合 GetConfigForClient 返回 errClientHelloRead，确保握手在适当的时候中断。
//
//		┌── readOnlyConn ──────────┐
//		│  Read  → PeekRead(conn)  │ ← 读取并缓存到 peeked
//		│  Write → io.EOF          │ ← 阻止写操作
//		│  其他  → panic (未实现)   │ ← 如果 TLS 握手调用了意外的方法
//		└──────────────────────────┘
type readOnlyConn struct {
	net.Conn // 嵌入 net.Conn，只覆盖 Read 和 Write 方法

	conn *peekConn // 实际的数据来源
}

// Read 从 peekConn 读取数据（使用 PeekRead 以保证字节被缓存到 peeked 缓冲区）。
//
// PeekRead 与标准 Read 的区别：
//   - 标准 Read：从 bufio.Reader 消费字节，消费后无法重放
//   - PeekRead：从 bufio.Reader 消费字节的同时拷贝到 peeked 缓冲区
//   - 后续 peekConn.Read 会优先返回 peeked 中的缓存 → 实现零字节丢失
func (c readOnlyConn) Read(p []byte) (int, error) {
	return c.conn.PeekRead(p)
}

// Write 始终返回失败，防止假的 TLS 握手往连接上写任何数据。
// 返回 io.EOF 是最安全的方式，让 TLS 库认为对端已关闭，停止握手。
func (readOnlyConn) Write(_ []byte) (int, error) {
	return 0, io.EOF
}
