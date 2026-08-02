package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/andrewbytecoder/nmq/internal/config/dynamic"
	"github.com/andrewbytecoder/nmq/pkg/types"
	"github.com/pires/go-proxyproto"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// ClientConn 表示进入 nmq 的客户端连接信息。
// 当 nmq 作为 TCP 反向代理时，需要知道原始客户端的地址，
// 以便在向目标后端发送 PROXY Protocol 头时填写正确的客户端 IP 和端口。
type ClientConn interface {
	// LocalAddr 返回本地网络地址（即 nmq 接收连接的地址）。
	LocalAddr() net.Addr

	// RemoteAddr 返回远程网络地址（即真实客户端的地址）。
	RemoteAddr() net.Addr
}

// Dialer 定义了 TCP 代理层连接后端的统一接口。
// 所有连接（纯 TCP 与 TLS）都通过此接口发起，保证了底层 dialer 的可替换性。
type Dialer interface {
	// Dial 拨号连接，不带 context。
	Dial(network, addr string, client ClientConn) (c net.Conn, err error)

	// DialContext 拨号连接，携带 context，支持超时/取消控制。
	DialContext(ctx context.Context, network, addr string, client ClientConn) (c net.Conn, err error)

	// TerminationDelay 返回连接终结延迟时间，
	// 用于在一方关闭写端后等待一段时间再彻底关闭连接，避免数据丢失。
	TerminationDelay() time.Duration
}

// tcpDialer 是 Dialer 接口的纯 TCP 实现。
// 它在标准 net.Dialer 之上增加了 PROXY Protocol 支持和终结延迟。
type tcpDialer struct {
	dialer           *net.Dialer            // 底层 Go 标准拨号器
	terminationDelay time.Duration          // 连接终结前等待时间
	proxyProtocol    *dynamic.ProxyProtocol // PROXY Protocol 配置，nil 表示不发送
}

// TerminationDelay 返回配置的终结延迟。
func (d tcpDialer) TerminationDelay() time.Duration {
	return d.terminationDelay
}

// Dial 是对 DialContext 的无 context 包装，使用 context.Background()。
func (d tcpDialer) Dial(network, addr string, clientConn ClientConn) (c net.Conn, err error) {
	return d.DialContext(context.Background(), network, addr, clientConn)
}

// DialContext 建立到后端的 TCP 连接，并在连接成功后根据配置发送 PROXY Protocol 头。
//
// 流程：
//  1. 通过标准 net.Dialer 拨号
//  2. 如果配置了 PROXY Protocol，将原始客户端地址写入连接负载
//  3. 返回建立好的 TCP 连接
func (d tcpDialer) DialContext(ctx context.Context, network, addr string, clientConn ClientConn) (c net.Conn, err error) {
	// 第一步：使用标准拨号器建立 TCP 连接
	conn, err := d.dialer.DialContext(ctx, network, addr)
	// 判断拨号是否失败：如果失败直接返回错误，无需做后续处理
	if err != nil {
		return nil, err
	}

	// 第二步：判断是否需要发送 PROXY Protocol 头。
	// 同时满足以下四个条件才发送：
	//   - d.proxyProtocol != nil：配置中声明了 PROXY Protocol
	//   - clientConn != nil：存在原始客户端连接信息（如果为 nil 则无源地址可填）
	//   - d.proxyProtocol.Version > 0：版本号合法（非零值表示未设置）
	//   - d.proxyProtocol.Version <= 3：版本号在合法范围内（V1、V2 目前实际支持）
	// 以上任一条件不满足，则跳过 PROXY Protocol 发送，直接返回裸连接。
	if d.proxyProtocol != nil && clientConn != nil && d.proxyProtocol.Version > 0 && d.proxyProtocol.Version <= 3 {
		// 根据客户端地址构建 PROXY Protocol 头部
		header := proxyproto.HeaderProxyFromAddrs(byte(d.proxyProtocol.Version), clientConn.RemoteAddr(), clientConn.LocalAddr())
		// 将头部写入连接
		if _, err = header.WriteTo(conn); err != nil {
			// 写入 PROXY Protocol 头失败：
			// 此时连接已不可用，必须关闭已建立的连接，防止资源泄漏
			_ = conn.Close()
			return nil, fmt.Errorf("writing PROXY Protocol header: %w", err)
		}
	}

	return conn, nil
}

// tcpTLSDialer 在 tcpDialer 基础上增加 TLS 握手能力。
// 它嵌入（embedding） tcpDialer，复用其拨号和 PROXY Protocol 逻辑。
type tcpTLSDialer struct {
	tcpDialer // 嵌入纯 TCP dialer，继承其 Dial/DialContext 行为

	tlsConfig *tls.Config // TLS 客户端配置，nil 时表示不需要 TLS
}

// Dial 是对 DialContext 的无 context 包装。
func (d tcpTLSDialer) Dial(network, addr string, clientConn ClientConn) (c net.Conn, err error) {
	return d.DialContext(context.Background(), network, addr, clientConn)
}

// DialContext 先通过底层 tcpDialer 建立连接，再执行 TLS 握手。
//
// 流程：
//  1. 调用嵌入的 tcpDialer.DialContext 完成 TCP 连接 + PROXY Protocol
//  2. 在 TCP 连接之上执行客户端 TLS 握手
//  3. 返回加密的 TLS 连接
func (d tcpTLSDialer) DialContext(ctx context.Context, network, addr string, clientConn ClientConn) (c net.Conn, err error) {
	// 第一步：完成 TCP 层连接（含 PROXY Protocol）
	conn, err := d.tcpDialer.DialContext(ctx, network, addr, clientConn)
	// 判断 TCP 连接是否失败：失败则直接返回，无需进行 TLS 握手
	if err != nil {
		return nil, err
	}

	// 第二步：在 TCP 连接之上包装 TLS 客户端
	tlsConn := tls.Client(conn, d.tlsConfig)
	// 第三步：执行 TLS 握手，验证后端证书（或 SPIFFE 身份）
	if err = tlsConn.Handshake(); err != nil {
		// TLS 握手失败：
		// 底层 TCP 连接虽然已建立，但因握手不成功，连接无效，
		// 必须关闭以避免泄漏
		_ = conn.Close()
		return nil, fmt.Errorf("TLS handshake: %w", err)
	}

	return tlsConn, nil
}

// SpiffeX509Source 定义了 SPIFFE X.509 SVID 源接口。
// 它组合了 SVID 来源和 Bundle 来源两个能力：
//   - x509svid.Source：提供自身的 SPIFFE SVID（客户端证书）
//   - x509bundle.Source：提供信任域 Bundle（用于验证对端证书）
//
// 在构建 SPIFFE mTLS 连接时，DialerManager 需要同时具备这两个能力。
type SpiffeX509Source interface {
	x509svid.Source
	x509bundle.Source
}

// DialerManager 管理所有 TCP 后端连接的拨号器。
// 它持有所有已配置的 TCPServersTransport 及其对应的 SPIFFE 身份源，
// 在运行时为每个负载均衡配置创建合适的 Dialer。
type DialerManager struct {
	serversTransportsMu sync.RWMutex                            // 读写锁，保护 serversTransports 的并发访问
	serversTransports   map[string]*dynamic.TCPServersTransport // 按名称索引的 transport 配置
	spiffeX509Source    SpiffeX509Source                        // SPIFFE 身份源，nil 表示未启用 SPIFFE
}

// NewDialerManager 创建一个新的 DialerManager 实例。
// spiffeX509Source 可以为 nil，表示不使用 SPIFFE 认证。
func NewDialerManager(spiffeX509Source SpiffeX509Source) *DialerManager {
	return &DialerManager{
		serversTransports: make(map[string]*dynamic.TCPServersTransport),
		spiffeX509Source:  spiffeX509Source,
	}
}

// Update 全量替换当前所有 TCPServersTransport 配置。
// 此方法在动态配置更新时被调用，以线程安全的方式刷新 transport 注册表。
func (d *DialerManager) Update(configs map[string]*dynamic.TCPServersTransport) {
	d.serversTransportsMu.Lock()
	defer d.serversTransportsMu.Unlock()

	d.serversTransports = configs
}

// Build 根据 TCP 负载均衡配置构建对应的 Dialer。
//
// 参数：
//   - config：负载均衡配置，包含 transport 名称、终结延迟、PROXY Protocol 等
//   - isTLS：是否需要建立 TLS 连接。为 true 时返回 tcpTLSDialer，否则返回 tcpDialer
//
// 返回值：构建好的 Dialer，或构建过程中的错误
func (d *DialerManager) Build(config *dynamic.TCPServersLoadBalancer, isTLS bool) (Dialer, error) {
	// 确定使用的 transport 名称，未指定时使用内置默认值
	name := "default@internal"
	// 判断：是否显式指定了 serversTransport 名称
	if config.ServersTransport != "" {
		name = config.ServersTransport
	}

	// 从注册表中查找对应的 transport 配置
	var st *dynamic.TCPServersTransport
	d.serversTransportsMu.RLock()
	st, ok := d.serversTransports[name]
	d.serversTransportsMu.RUnlock()
	// 判断：transport 是否存在于注册表中
	// ok == false：名称未找到
	// st == nil：找到了但值为 nil（配置被删除等异常情况）
	if !ok || st == nil {
		return nil, fmt.Errorf("servers transport %s not found", name)
	}

	// ---------- TerminationDelay 与 ProxyProtocol 取值 ----------
	// 优先级：transport 级别 > service 级别。
	// 先取 service 级别的值作为默认，再被 transport 级别覆盖。

	var terminationDelay types.Duration
	// 判断：service 级别是否设置了 TerminationDelay
	if config.TerminationDelay != nil {
		terminationDelay = types.Duration(*config.TerminationDelay)
	}

	proxyProtocol := config.ProxyProtocol

	// 判断：是否指定了 serversTransport。
	// 如果指定，则使用 transport 级别的 TerminationDelay 和 ProxyProtocol 覆盖 service 级别的值。
	if config.ServersTransport != "" {
		terminationDelay = st.TerminationDelay
		proxyProtocol = st.ProxyProtocol
	}

	// 校验 PROXY Protocol 版本号：
	// Version 必须为 0（不发送）、1（V1 格式）或 2（V2 格式）。
	// Version < 1 或 Version > 2 均为非法值。
	// proxyProtocol != nil 确保只在配置了 PROXY Protocol 时才做校验，
	// 避免 nil 指针导致 panic。
	if proxyProtocol != nil && (proxyProtocol.Version < 1 || proxyProtocol.Version > 2) {
		return nil, fmt.Errorf("unknown proxyProtocol version: %d", proxyProtocol.Version)
	}

	// ---------- TLS 配置构建 ----------
	var tlsConfig *tls.Config
	// 判断：transport 是否配置了 TLS。
	// st.TLS 为 nil 时跳过所有 TLS 逻辑，使用 nil tlsConfig（纯 TCP 连接）。
	if st.TLS != nil {
		// --- 分支 A：SPIFFE 认证 ---
		// 判断：是否配置了 SPIFFE 身份认证。
		// SPIFFE 优先级最高，先处理。如果同时配置了传统 TLS 字段，
		// 在后续 if 中会检测到冲突并报错。
		if st.TLS.Spiffe != nil {
			// 判断：全局 SPIFFE Source 是否已配置。
			// 如果在 serversTransport 中启用了 SPIFFE，但 nmq 自身未配置
			// spiffe.workloadAPIAddr（spiffeX509Source 为 nil），
			// 则无法获取自身身份证书，必须报错。
			if d.spiffeX509Source == nil {
				return nil, errors.New("SPIFFE is enabled for this transport, but not configured")
			}

			// 构建 SPIFFE 身份验证器，决定哪些对端 SPIFFE ID 被信任
			authorizer, err := buildSpiffeAuthorizer(st.TLS.Spiffe)
			// 判断：验证器是否构建成功。
			// 可能失败的原因：SPIFFE ID 格式非法、trustDomain 格式非法等。
			if err != nil {
				return nil, fmt.Errorf("unable to build SPIFFE authorizer: %w", err)
			}

			// 构建 SPIFFE mTLS 配置：
			//   - d.spiffeX509Source 作为客户端证书源
			//   - d.spiffeX509Source 作为信任域 Bundle 源
			//   - authorizer 用于验证对端 SPIFFE 身份
			tlsConfig = tlsconfig.MTLSClientConfig(d.spiffeX509Source, d.spiffeX509Source, authorizer)
		}

		// --- 分支 B：传统 TLS 认证 ---
		// 判断：是否配置了任何传统 TLS 字段。
		// 这些字段包括：
		//   - InsecureSkipVerify：跳过证书验证
		//   - RootCAs：自定义 CA 证书池
		//   - ServerName：TLS SNI 服务器名
		//   - Certificates：客户端证书（mTLS）
		//   - PeerCertURI：对端证书 URI SAN 校验
		//   - PeerCertSANs：对端证书 SAN 校验
		// 只要有任意一个传统字段被设置，就进入传统 TLS 分支。
		if st.TLS.InsecureSkipVerify || len(st.TLS.RootCAs) > 0 || len(st.TLS.ServerName) > 0 || len(st.TLS.Certificates) > 0 || st.TLS.PeerCertURI != "" || len(st.TLS.PeerCertSANs) > 0 {
			// 冲突检测：
			// 如果 tlsConfig 已被 SPIFFE 分支（分支 A）设置，
			// 说明用户同时配置了 SPIFFE 和传统 TLS 字段，这是不允许的。
			// SPIFFE 和传统 TLS 是互斥的两种认证方式，必须二选一。
			if tlsConfig != nil {
				return nil, errors.New("TLS and SPIFFE configuration cannot be defined at the same time")
			}

			// 构建传统 TLS 配置：
			//   - ServerName：用于 SNI 和主机名验证
			//   - InsecureSkipVerify：跳过所有验证（仅适用于测试/内网环境）
			//   - RootCAs：自定义根证书池，用于验证后端自签名证书
			//   - Certificates：客户端证书，用于 mTLS
			tlsConfig = &tls.Config{
				ServerName:         st.TLS.ServerName,
				InsecureSkipVerify: st.TLS.InsecureSkipVerify,
				RootCAs:            createRootCACertPool(st.TLS.RootCAs),
				Certificates:       st.TLS.Certificates.GetCertificates(),
			}

			// 收集所有需要验证的对端证书 SAN。
			// 先从 PeerCertSANs 复制一份，避免修改原始配置数据。
			peerCertSANs := make([]types.SAN, len(st.TLS.PeerCertSANs))
			copy(peerCertSANs, st.TLS.PeerCertSANs)

			// 判断：是否配置了 PeerCertURI（URI 类型 SAN 校验，已废弃）。
			// 如果设置了 PeerCertURI，将其包装成 URI 型 SAN 并追加到校验列表。
			if st.TLS.PeerCertURI != "" {
				peerCertSANs = append(peerCertSANs, types.SAN{
					Type:  types.SANURIType,
					Value: st.TLS.PeerCertURI,
				})
			}

			// 判断：是否需要额外校验对端证书的 SAN。
			// 只有当 peerCertSANs 列表非空时，才设置自定义的 VerifyPeerCertificate 回调。
			// 该回调在标准 TLS 证书链验证通过后执行，额外检查对端证书的 SAN 字段
			// 是否匹配配置中指定的值。
			if len(peerCertSANs) > 0 {
				tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					return types.VerifyPeerCertificate(peerCertSANs, tlsConfig.RootCAs, rawCerts)
				}
			}
		}
	}

	// ---------- 构建最终的 Dialer ----------
	dialer := tcpDialer{
		dialer: &net.Dialer{
			Timeout:   time.Duration(st.DialTimeout),   // 连接超时
			KeepAlive: time.Duration(st.DialKeepAlive), // TCP Keep-Alive 间隔
		},
		terminationDelay: time.Duration(terminationDelay), // 终结延迟
		proxyProtocol:    proxyProtocol,                   // PROXY Protocol 配置
	}

	// 判断：是否需要 TLS。
	// 如果路由层已确定该连接需要 TLS，则返回 tcpTLSDialer 包装层，
	// 在 TCP 连接之上自动执行 TLS 握手。
	// 否则返回纯 tcpDialer，直接复用 TCP 连接。
	if !isTLS {
		return dialer, nil
	}
	return tcpTLSDialer{dialer, tlsConfig}, nil
}

// createRootCACertPool 从配置中的文件或内容读取 CA 证书，构建 x509.CertPool。
//
// 参数 rootCAs 为空时返回 nil，表示不自定义 CA 池，使用系统默认。
// 读取失败的证书会被静默跳过（continue），只加载成功解析的证书。
func createRootCACertPool(rootCAs []types.FileOrContent) *x509.CertPool {
	// 判断：CA 列表是否为空。
	// 空列表表示不需要自定义根证书池，返回 nil 让 TLS 使用系统默认。
	if len(rootCAs) == 0 {
		return nil
	}

	roots := x509.NewCertPool()
	for _, cert := range rootCAs {
		certContent, err := cert.Read()
		// 判断：读取 CA 证书内容是否失败。
		// 失败时静默跳过该证书，继续处理下一个。
		// 不会因为单个证书加载失败而中断所有证书的加载。
		if err != nil {
			continue
		}
		roots.AppendCertsFromPEM(certContent)
	}

	return roots
}

// buildSpiffeAuthorizer 根据 SPIFFE 配置构建对端身份验证器。
//
// 三种认证模式（按优先级排序）：
//  1. IDs 白名单模式（最高优先级）：只允许列表中精确的 SPIFFE ID
//  2. TrustDomain 信任域模式：允许指定域下的所有 SPIFFE ID
//  3. default 兜底模式：接受任何 SPIFFE 身份（安全级别最低）
func buildSpiffeAuthorizer(cfg *dynamic.Spiffe) (tlsconfig.Authorizer, error) {
	switch {
	// 情况 1：配置了白名单 SPIFFE ID 列表。
	// 例如 ids: ["spiffe://example.org/db", "spiffe://example.org/cache"]
	// 后端的 SPIFFE ID 必须精确匹配列表中的某个值。
	// 这是最严格、安全性最高的模式。
	case len(cfg.IDs) > 0:
		spiffeIDs := make([]spiffeid.ID, 0, len(cfg.IDs))
		for _, rawID := range cfg.IDs {
			id, err := spiffeid.FromString(rawID)
			// 判断：SPIFFE ID 格式是否合法。
			// 非法格式（如非 spiffe:// 前缀、空字符串等）应立即报错，
			// 而非静默跳过，因为用户很可能配置错误。
			if err != nil {
				return nil, fmt.Errorf("invalid SPIFFE ID: %w", err)
			}
			spiffeIDs = append(spiffeIDs, id)
		}
		return tlsconfig.AuthorizeOneOf(spiffeIDs...), nil

	// 情况 2：未指定白名单，但配置了信任域。
	// 例如 trustDomain: "example.org"
	// 允许 spiffe://example.org/ 下的任意 SPIFFE ID。
	// 适用于同一信任域下所有服务都需要访问的场景。
	case cfg.TrustDomain != "":
		trustDomain, err := spiffeid.TrustDomainFromString(cfg.TrustDomain)
		// 判断：信任域格式是否合法。
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE trust domain: %w", err)
		}

		return tlsconfig.AuthorizeMemberOf(trustDomain), nil

	// 情况 3：既无白名单也无信任域。
	// 此时不做 SPIFFE 身份校验，接受任何 SPIFFE 身份（甚至无身份）。
	// 安全级别最低，等同于仅使用 TLS 加密但不做身份验证。
	default:
		return tlsconfig.AuthorizeAny(), nil
	}
}
