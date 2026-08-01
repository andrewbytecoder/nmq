package tcp

import "net"

// Handler is a TCP handler interface.
type Handler interface {
	ServeTCP(conn WriteCloser)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as TCP handlers.
// 允许使用普通函数作为TCP处理程序。
type HandlerFunc func(conn WriteCloser)

// ServeTCP serves TCP.
func (f HandlerFunc) ServeTCP(conn WriteCloser) {
	f(conn)
}

type WriteCloser interface {
	net.Conn

	// CloseWrite closes on a network connection, indicates that the issuer of the call
	// has terminated sending on that connection.
	// It corresponds to sending a FIN packet
	CloseWrite() error
}
