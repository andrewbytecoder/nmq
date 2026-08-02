package dynamic

import (
	"github.com/andrewbytecoder/nmq/pkg/types"
)

// +k8s:deepcopy-gen=true

// ProxyProtocol holds the PROXY Protocol configuration.
// More info: https://doc.traefik.io/traefik/v3.7/routing/services/#proxy-protocol
type ProxyProtocol struct {
	// Version defines the PROXY Protocol version to use.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2
	Version int `description:"Defines the PROXY Protocol version to use." json:"version,omitempty" toml:"version,omitempty" yaml:"version,omitempty" export:"true"`
}

// SetDefaults Default values for a ProxyProtocol.
func (p *ProxyProtocol) SetDefaults() {
	p.Version = 2
}

// +k8s:deepcopy-gen=true

// TCPServersTransport options to configure communication between Traefik and the servers.
type TCPServersTransport struct {
	DialKeepAlive types.Duration `description:"Defines the interval between keep-alive probes for an active network connection. If zero, keep-alive probes are sent with a default value (currently 15 seconds), if supported by the protocol and operating system. Network protocols or operating systems that do not support keep-alives ignore this field. If negative, keep-alive probes are disabled" json:"dialKeepAlive,omitempty" toml:"dialKeepAlive,omitempty" yaml:"dialKeepAlive,omitempty" export:"true"`
	DialTimeout   types.Duration `description:"Defines the amount of time to wait until a connection to a backend server can be established. If zero, no timeout exists." json:"dialTimeout,omitempty" toml:"dialTimeout,omitempty" yaml:"dialTimeout,omitempty" export:"true"`
	// ProxyProtocol holds the PROXY Protocol configuration.
	ProxyProtocol *ProxyProtocol `description:"Defines the PROXY Protocol configuration." json:"proxyProtocol,omitempty" toml:"proxyProtocol,omitempty" yaml:"proxyProtocol,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
	// TerminationDelay, corresponds to the deadline that the proxy sets, after one
	// of its connected peers indicates it has closed the writing capability of its
	// connection, to close the reading capability as well, hence fully terminating the
	// connection. It is a duration in milliseconds, defaulting to 100. A negative value
	// means an infinite deadline (i.e. the reading capability is never closed).
	TerminationDelay types.Duration   `description:"Defines the delay to wait before fully terminating the connection, after one connected peer has closed its writing capability." json:"terminationDelay,omitempty" toml:"terminationDelay,omitempty" yaml:"terminationDelay,omitempty" export:"true"`
	TLS              *TLSClientConfig `description:"Defines the TLS configuration." json:"tls,omitempty" toml:"tls,omitempty" yaml:"tls,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
}

// +k8s:deepcopy-gen=true

// TLSClientConfig options to configure TLS communication between Traefik and the servers.
type TLSClientConfig struct {
	ServerName         string                `description:"Defines the serverName used to contact the server." json:"serverName,omitempty" toml:"serverName,omitempty" yaml:"serverName,omitempty"`
	InsecureSkipVerify bool                  `description:"Disables SSL certificate verification." json:"insecureSkipVerify,omitempty" toml:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty" export:"true"`
	RootCAs            []types.FileOrContent `description:"Defines a list of CA certificates used to validate server certificates." json:"rootCAs,omitempty" toml:"rootCAs,omitempty" yaml:"rootCAs,omitempty"`
	Certificates       types.Certificates    `description:"Defines a list of client certificates for mTLS." json:"certificates,omitempty" toml:"certificates,omitempty" yaml:"certificates,omitempty" export:"true"`
	// Deprecated: PeerCertURI is deprecated, please use the PeerCertSANs option instead.
	PeerCertURI  string      `description:"Defines the URI used to match against SAN URI during the peer certificate verification." json:"peerCertURI,omitempty" toml:"peerCertURI,omitempty" yaml:"peerCertURI,omitempty"`
	PeerCertSANs []types.SAN `description:"Defines the SANs (Subject Alternative Names) used to match against SANs during the peer certificate verification." json:"peerCertSANs,omitempty" toml:"peerCertSANs,omitempty" yaml:"peerCertSANs,omitempty"`
	Spiffe       *Spiffe     `description:"Defines the SPIFFE TLS configuration." json:"spiffe,omitempty" toml:"spiffe,omitempty" yaml:"spiffe,omitempty" label:"allowEmpty" file:"allowEmpty" export:"true"`
}

// +k8s:deepcopy-gen=true

// TCPServer holds a TCP Server configuration.
type TCPServer struct {
	Address string `json:"address,omitempty" toml:"address,omitempty" yaml:"address,omitempty" label:"-"`
	Port    string `json:"-" toml:"-" yaml:"-"`
	TLS     bool   `json:"tls,omitempty" toml:"tls,omitempty" yaml:"tls,omitempty"`
}

// +k8s:deepcopy-gen=true

// TCPServersLoadBalancer holds the LoadBalancerService configuration.
type TCPServersLoadBalancer struct {
	Servers          []TCPServer `json:"servers,omitempty" toml:"servers,omitempty" yaml:"servers,omitempty" label-slice-as-struct:"server" export:"true"`
	ServersTransport string      `json:"serversTransport,omitempty" toml:"serversTransport,omitempty" yaml:"serversTransport,omitempty" export:"true"`
	// ProxyProtocol holds the PROXY Protocol configuration.
	//
	// Deprecated: use ServersTransport to configure ProxyProtocol instead.
	ProxyProtocol *ProxyProtocol `json:"proxyProtocol,omitempty" toml:"proxyProtocol,omitempty" yaml:"proxyProtocol,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
	// TerminationDelay, corresponds to the deadline that the proxy sets, after one
	// of its connected peers indicates it has closed the writing capability of its
	// connection, to close the reading capability as well, hence fully terminating the
	// connection. It is a duration in milliseconds, defaulting to 100. A negative value
	// means an infinite deadline (i.e. the reading capability is never closed).
	//
	// Deprecated: use ServersTransport to configure the TerminationDelay instead.
	TerminationDelay *int                  `json:"terminationDelay,omitempty" toml:"terminationDelay,omitempty" yaml:"terminationDelay,omitempty" export:"true"`
	HealthCheck      *TCPServerHealthCheck `json:"healthCheck,omitempty" toml:"healthCheck,omitempty" yaml:"healthCheck,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
}

// +k8s:deepcopy-gen=true

// TCPServerHealthCheck holds the HealthCheck configuration.
type TCPServerHealthCheck struct {
	Port              int             `json:"port,omitempty" toml:"port,omitempty,omitzero" yaml:"port,omitempty" export:"true"`
	Send              string          `json:"send,omitempty" toml:"send,omitempty" yaml:"send,omitempty" export:"true"`
	Expect            string          `json:"expect,omitempty" toml:"expect,omitempty" yaml:"expect,omitempty" export:"true"`
	Interval          types.Duration  `json:"interval,omitempty" toml:"interval,omitempty" yaml:"interval,omitempty" export:"true"`
	UnhealthyInterval *types.Duration `json:"unhealthyInterval,omitempty" toml:"unhealthyInterval,omitempty" yaml:"unhealthyInterval,omitempty" export:"true"`
	Timeout           types.Duration  `json:"timeout,omitempty" toml:"timeout,omitempty" yaml:"timeout,omitempty" export:"true"`
}

//
