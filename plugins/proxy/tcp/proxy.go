package tcp

import (
	"errors"
	"io"
	"net"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Proxy 是 TCP 代理的核心结构体。
// 它接收客户端的 TCP 连接，将其转发到指定的后端地址，
// 并在客户端和后端之间双向拷贝数据。
type Proxy struct {
	log *zap.Logger // 日志记录器

	address string // 目标后端地址，格式为 "host:port"
	dialer  Dialer // 后端拨号器，支持 PROXY Protocol 和 TLS
}

// NewProxy 创建一个新的 TCP 代理实例。
// address 是后端服务器地址，dialer 负责建立到后端的连接。
func NewProxy(address string, dialer Dialer, log *zap.Logger) (*Proxy, error) {
	return &Proxy{
		address: address,
		dialer:  dialer,
		log:     log,
	}, nil
}

// ServeTCP 执行 TCP 代理的核心逻辑：建立到后端的连接，然后在客户端和后端之间双向转发数据。
//
// 流程：
//  1. 拨号连接后端
//  2. 启动两个 goroutine 双向拷贝数据
//  3. 等待拷贝结束（其中一个方向先结束）
//  4. connCopy 内部会触发优雅关闭（FIN + TerminationDelay）
//  5. 等待另一个方向也结束
//  6. 返回
func (p *Proxy) ServeTCP(conn WriteCloser) error {
	p.log.Debug("ServeTCP", zap.String("remote", conn.RemoteAddr().String()),
		zap.String("address", p.address),
		zap.String("local", conn.LocalAddr().String()))

	// 确保客户端连接在 ServeTCP 返回后被关闭。
	// 例如当连接被 server.trackedConnection 追踪时，需要此 defer 来释放资源。
	defer conn.Close()

	// 第一步：拨号到后端服务器。
	// 传入客户端连接对象，以便 dialer 可以从中提取客户端地址信息
	// 构建 PROXY Protocol 头。
	connBackend, err := p.dialBackend(conn)
	// 判断拨号是否失败：失败时记录错误并返回，客户端连接由 defer conn.Close() 关闭。
	if err != nil {
		p.log.Error("dial backend error", zap.Error(err))
		return err
	}
	// 确保后端连接在 ServeTCP 返回后被关闭。
	// 正常情况下后端连接会在 connCopy 中被 CloseWrite 关闭，
	// 此 defer 作为兜底，防止异常路径下的连接泄漏。
	defer connBackend.Close()

	// 第二步：双向数据拷贝。
	// 使用无缓冲 channel 在 goroutine 间传递拷贝结果。
	// 无缓冲确保发送方在接收方准备好之前阻塞，保证两个方向的错误都能被收集。
	errChan := make(chan error)

	// goroutine A：客户端 → 后端（上行数据）
	// src=conn(客户端), dst=connBackend(后端)
	go p.connCopy(conn, connBackend, errChan)

	// goroutine B：后端 → 客户端（下行数据）
	// src=connBackend(后端), dst=conn(客户端)
	go p.connCopy(connBackend, conn, errChan)

	// 第三步：等待第一个方向的数据拷贝结束。
	// 先结束的方向可能是任意一侧（客户端先断开、后端先响应完等），
	// ServeTCP 不关心谁先结束，只关心接收第一个错误通知。
	err = <-errChan
	// 判断第一个结束的方向是否有错误。
	// 注意：连接被重置（Connection Reset）是一个合法的 TCP 结束方式，
	// 对端发送 RST 包只是表示"不再发送数据"，不应视为错误。
	if err != nil {
		// 判断是否为读取过程中的连接重置错误（RST）。
		// 平台相关：Unix 为 syscall.ECONNRESET，Windows 为 syscall.WSAECONNRESET。
		// 这种错误是对端的正常断开行为，不是代理的错误，因此：
		//   - 不记录日志（避免告警噪音）
		//   - 不做额外处理
		// 其他类型的错误没有在此处理，调用方可扩展 logging 逻辑。
		if isReadConnResetError(err) {
			// RST 错误，静默处理，不记录日志
		} else {
			// 预留：其他类型错误可在此扩展处理
		}
	}

	// 第四步：等待第二个方向的数据拷贝结束。
	// 当两个 goroutine 都结束后，ServeTCP 才能安全返回。
	// 第一个方向的 CloseWrite 通常会触发第二个方向的 io.Copy 结束。
	<-errChan

	// ServeTCP 返回后，两个 defer 会分别关闭客户端和后端连接。
	return nil
}

// dialBackend 通过 Dialer 拨号到后端地址。
// clientConn 被透传给 Dialer，使其能够提取原始客户端地址
// 用于构建 PROXY Protocol 头（如果启用了 PROXY Protocol）。
func (p *Proxy) dialBackend(clientConn net.Conn) (WriteCloser, error) {
	// 将客户端连接传递给 dialer。
	// dialer 内部的 tcpDialer.DialContext 会利用 clientConn 的 RemoteAddr/LocalAddr
	// 来填充 PROXY Protocol 头的源地址和目标地址字段。
	conn, err := p.dialer.Dial("tcp", p.address, clientConn)
	// 判断拨号是否失败：失败时直接返回错误。
	// 错误原因可能是：后端不可达、连接超时、TLS 握手失败、SPIFFE 身份验证失败等。
	if err != nil {
		return nil, err
	}

	// 将返回的 net.Conn 断言为 WriteCloser。
	// WriteCloser 接口在 handler.go 中定义，扩展了 net.Conn 并增加了 CloseWrite() 方法，
	// 用于 TCP 半关闭（发送 FIN 包但保持读端打开）。
	// tcpDialer 和 tcpTLSDialer 返回的连接都实现了 WriteCloser。
	return conn.(WriteCloser), nil
}

// connCopy 在 dst 和 src 之间执行双向数据拷贝中的一个方向：从 src 读取，写入 dst。
//
// 拷贝结束后执行 TCP 优雅关闭流程：
//  1. 将 io.Copy 的返回错误发送到 errCh（通知 ServeTCP 主 goroutine）
//  2. 调用 dst.CloseWrite() 发送 FIN 包，告知对端"我不再发送数据了"
//  3. 如果配置了 TerminationDelay，设置读超时，在延迟时间内继续接收对端最后的响应数据
//
// 参数：
//   - dst：写入目标（拷贝方向的接收方）
//   - src：读取来源（拷贝方向的发送方）
//   - errCh：错误通知 channel
func (p *Proxy) connCopy(dst, src WriteCloser, errCh chan error) {
	// ---------- 阶段一：数据拷贝 ----------
	// io.Copy 会把 src 的所有数据拷贝到 dst，直到 src 返回 EOF 或错误。
	// 典型情况：
	//   - src 端调用 close() → src 的 Read 返回 EOF → io.Copy 正常结束
	//   - 网络中断 → src 的 Read 返回错误 → io.Copy 异常结束
	_, err := io.Copy(dst, src)

	// 将拷贝结果通知主 goroutine。
	// 注意：errCh 是无缓冲 channel，如果主 goroutine 还没开始等待第二个错误，
	// 此 goroutine 会在此阻塞，这是符合预期的行为。
	errCh <- err

	// ---------- 阶段二：发送 FIN（优雅关闭） ----------
	// CloseWrite() 是 TCP 半关闭操作，发送 FIN 包给 dst 对端，
	// 表示"我已发完所有数据"。
	// 此时连接并没有完全关闭——读端仍然打开，可以继续接收对端的响应数据。
	errClose := dst.CloseWrite()
	// 判断 CloseWrite 是否出错。
	if errClose != nil {
		// CloseWrite 在以下情况会失败：
		//   1. socket 已处于 "not connected" 状态——常见于对端先发送了 RST 包
		//      （另一个 goroutine 的 connCopy 中 dst 连接已被 RST 中断）
		//   2. 其他底层错误
		//
		// 情况 1 是预期行为，不需要记录日志，因为：
		//   - 连接已被对端 RST 关闭，CloseWrite 本身就多余
		//   - 记录日志只会产生噪音
		//
		// 判断是否为 socket 未连接错误（syscall.ENOTCONN / syscall.WSAENOTCONN）。
		// 如果不是预期错误，则记录日志。
		if !isSocketNotConnectedError(errClose) {
			p.log.Error("error while closing the connection", zap.Error(errClose))
		}

		// 无论是什么错误，CloseWrite 失败后都不再需要 TerminationDelay，
		// 因为连接已经不可用（无法继续读数据），直接返回。
		return
	}

	// ---------- 阶段三：TerminationDelay 等待最后的数据 ----------
	// CloseWrite 成功后，连接处于半关闭状态（写端关闭，读端打开）。
	// 给 dst 连接设置读超时（ReadDeadline），在此时间内继续接收
	// 对端可能发送的最后数据（例如 HTTP 响应尾、TLS close_notify 等）。
	// 这是实现 TCP 优雅关闭的关键步骤。
	//
	// 判断 TerminationDelay 是否 >= 0：
	//   - >= 0：启用优雅关闭，设置读超时为 "当前时间 + TerminationDelay"
	//   - < 0（通常是 -1）：禁用优雅关闭，不设置读超时。
	//     设置负数会导致对端立即收到 Close 而非半关闭。
	if p.dialer.TerminationDelay() >= 0 {
		// 设置读超时时间 = 当前时间 + 终结延迟。
		// 在这段时间内，即使没有数据到达，Read 操作也会在超时后返回，
		// 从而触发连接完全关闭（上方 ServeTCP 的 defer conn.Close()）。
		err = dst.SetReadDeadline(time.Now().Add(p.dialer.TerminationDelay()))
		// 判断 SetReadDeadline 是否失败。
		// 失败原因通常是底层连接已经不可用（已被对方关闭等）。
		// 这种情况下优雅关闭无法完成，记录错误但不影响流程。
		if err != nil {
			p.log.Error("error while setting read deadline", zap.Error(err))
		}
	}
}

// isSocketNotConnectedError 判断错误是否由 socket 未连接（ENOTCONN）引起。
//
// 当对端发送 RST 包终止连接时，已失效的 socket 上调用 CloseWrite
// 会返回该错误。这是预期的 TCP 行为，不是代理异常，因此被调用方静默忽略。
//
// 两步判断：
//  1. 错误类型必须是 *net.OpError（网络操作错误）
//  2. 底层错误必须包装了 syscall.ENOTCONN
//
// 两个条件同时满足才认为是 socket 未连接错误。
func isSocketNotConnectedError(err error) bool {
	// 判断是否为 *net.OpError 类型。
	// errors.AsType 是 Go 1.23+ 的类型推断式断言，等价于 errors.As。
	// 只有 *net.OpError 才可能是网络操作错误，其他类型直接排除。
	_, ok := errors.AsType[*net.OpError](err)

	// 判断类型匹配 && 底层错误是 syscall.ENOTCONN。
	// errors.Is 会沿着错误链（wrapping chain）查找 ENOTCONN。
	// 两个条件同时为 true 才返回 true：
	//   - ok == false → 不是网络操作错误 → 返回 false
	//   - ok == true 但非 ENOTCONN → 返回 false（是其他网络错误，需要记录日志）
	return ok && errors.Is(err, syscall.ENOTCONN)
}
