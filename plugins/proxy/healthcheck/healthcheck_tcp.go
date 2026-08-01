package healthcheck

// maxPayloadSize is the maximum payload size that can be sent during health checks.
const maxPayloadSize = 65535

type TCPHealthCheckTarget struct {
	Address string
	TLS     bool
	Dialer  tcp.Dialer
}
