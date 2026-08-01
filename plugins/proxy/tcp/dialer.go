package tcp

import "net"

type ClientConn interface {
	LocalAddr() net.Addr

	RemoteAddr() net.Addr
}
