package tcp

import (
	"crypto/tls"
	"errors"
	"net/http"

	tcpmuxer "github.com/andrewbytecoder/nmq/plugins/proxy/muxer/tcp"
	"github.com/andrewbytecoder/nmq/plugins/proxy/tcp"
)

// errClientHelloRead is used as a sentinel error to break the TLS handshake once we have read the ClientHello.
var errClientHelloRead = errors.New("client hello successfully read")

type tlsConfigWithOptionsName struct {
	cfg         *tls.Config
	optionsName string
}

// Router is a TCP router.
type Router struct {
	acmeTLSPassthrough bool

	// Contains TCP routes.
	muxerTCP tcpmuxer.Muxer
	// Contains TCP TLS routes.
	muxerTCPTLS tcpmuxer.Muxer
	// Contains HTTPS routes.
	muxerHTTPS tcpmuxer.Muxer

	// Forwarder handlers.
	// httpForwarder handles all HTTP requests.
	httpForwarder tcp.Handler
	// httpsForwarder handles (indirectly through muxerHTTPS, or directly) all HTTPS requests.
	httpsForwarder tcp.Handler

	// Neither is used directly, but they are held here, and recreated on config reload,
	// so that they can be passed to the Switcher at the end of the config reload phase.
	httpHandler  http.Handler
	httpsHandler http.Handler

	// TLS configs.
	httpsTLSConfig *tls.Config // default TLS config
	// hostHTTPTLSConfig contains TLS configs keyed by SNI.
	// A nil config is the hint to set up a brokenTLSRouter.
	hostHTTPTLSConfig map[string]tlsConfigWithOptionsName // TLS configs keyed by SNI
}
