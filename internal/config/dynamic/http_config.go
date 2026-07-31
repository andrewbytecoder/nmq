package dynamic

import (
	"github.com/andrewbytecoder/nmq/pkg/types"
	"github.com/golang/protobuf/types"
)

// +k8s:deepcopy-gen=true

// Service holds a service configuration (can only be of one type at the same time).
type Service struct {
	Middlewares         []string             `json:"middlewares,omitempty" toml:"middlewares,omitempty" yaml:"middlewares,omitempty" export:"true"`
	LoadBalancer        *ServersLoadBalancer `json:"loadBalancer,omitempty" toml:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty" export:"true"`
	HighestRandomWeight *HighestRandomWeight `json:"highestRandomWeight,omitempty" toml:"highestRandomWeight,omitempty" yaml:"highestRandomWeight,omitempty" label:"-" export:"true"`
	Weighted            *WeightedRoundRobin  `json:"weighted,omitempty" toml:"weighted,omitempty" yaml:"weighted,omitempty" label:"-" export:"true"`
	Mirroring           *Mirroring           `json:"mirroring,omitempty" toml:"mirroring,omitempty" yaml:"mirroring,omitempty" label:"-" export:"true"`
	Failover            *Failover            `json:"failover,omitempty" toml:"failover,omitempty" yaml:"failover,omitempty" label:"-" export:"true"`
}

type BalancerStrategy string

const (
	// BalancerStrategyWRR is the weighted round-robin strategy.
	BalancerStrategyWRR BalancerStrategy = "wrr"
	// BalancerStrategyP2C is the power of two choices strategy.
	BalancerStrategyP2C BalancerStrategy = "p2c"
	// BalancerStrategyHRW is the highest random weight strategy.
	BalancerStrategyHRW BalancerStrategy = "hrw"
	// BalancerStrategyLeastTime is the least-time strategy.
	BalancerStrategyLeastTime BalancerStrategy = "leasttime"
)

// ServersLoadBalancer holds the ServersLoadBalancer configuration.
type ServersLoadBalancer struct {
	Sticky   *Sticky          `json:"sticky,omitempty" toml:"sticky,omitempty" yaml:"sticky,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
	Servers  []Server         `json:"servers,omitempty" toml:"servers,omitempty" yaml:"servers,omitempty" label-slice-as-struct:"server" export:"true"`
	Strategy BalancerStrategy `json:"strategy,omitempty" toml:"strategy,omitempty" yaml:"strategy,omitempty" export:"true"`
	// HealthCheck enables regular active checks of the responsiveness of the
	// children servers of this load-balancer. To propagate status changes (e.g. all
	// servers of this service are down) upwards, HealthCheck must also be enabled on
	// the parent(s) of this service.
	HealthCheck *ServerHealthCheck `json:"healthCheck,omitempty" toml:"healthCheck,omitempty" yaml:"healthCheck,omitempty" export:"true"`
	// PassiveHealthCheck enables passive health checks for children servers of this load-balancer.
	PassiveHealthCheck *PassiveServerHealthCheck `json:"passiveHealthCheck,omitempty" toml:"passiveHealthCheck,omitempty" yaml:"passiveHealthCheck,omitempty" export:"true"`
	PassHostHeader     *bool                     `json:"passHostHeader" toml:"passHostHeader" yaml:"passHostHeader" export:"true"`
	ResponseForwarding *ResponseForwarding       `json:"responseForwarding,omitempty" toml:"responseForwarding,omitempty" yaml:"responseForwarding,omitempty" export:"true"`
	ServersTransport   string                    `json:"serversTransport,omitempty" toml:"serversTransport,omitempty" yaml:"serversTransport,omitempty" export:"true"`

	// NginxUpstreamHashBy enables the customization of the hashing key.
	// It can be set to a specific text value, a NGINX variable or a combination of both.
	NginxUpstreamHashBy string `json:"nginxUpstreamHashBy,omitempty" toml:"-" yaml:"-" label:"-" file:"-" kv:"-" export:"true"`
}

// +k8s:deepcopy-gen=true

// HRWService is a reference to a service load-balanced with highest random weight.
type HRWService struct {
	Name   string `json:"name,omitempty" toml:"name,omitempty" yaml:"name,omitempty" export:"true"`
	Weight *int   `json:"weight,omitempty" toml:"weight,omitempty" yaml:"weight,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// HighestRandomWeight is a weighted sticky load-balancer of services.
type HighestRandomWeight struct {
	Services []HRWService `json:"services,omitempty" toml:"services,omitempty" yaml:"services,omitempty" export:"true"`
	// HealthCheck enables automatic self-healthcheck for this service, i.e.
	// whenever one of its children is reported as down, this service becomes aware of it,
	// and takes it into account (i.e. it ignores the down child) when running the
	// load-balancing algorithm. In addition, if the parent of this service also has
	// HealthCheck enabled, this service reports to its parent any status change.
	HealthCheck *HealthCheck `json:"healthCheck,omitempty" toml:"healthCheck,omitempty" yaml:"healthCheck,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
}

// +k8s:deepcopy-gen=true

// HealthCheck controls healthcheck awareness and propagation at the services level.
type HealthCheck struct{}

// +k8s:deepcopy-gen=true

type PassiveServerHealthCheck struct {
	// FailureWindow defines the time window during which the failed attempts must occur for the server to be marked as unhealthy. It also defines for how long the server will be considered unhealthy.
	FailureWindow types.Duration `json:"failureWindow,omitempty" toml:"failureWindow,omitempty" yaml:"failureWindow,omitempty" export:"true"`
	// MaxFailedAttempts is the number of consecutive failed attempts allowed within the failure window before marking the server as unhealthy.
	MaxFailedAttempts int `json:"maxFailedAttempts,omitempty" toml:"maxFailedAttempts,omitempty" yaml:"maxFailedAttempts,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// ResponseForwarding holds the response forwarding configuration.
type ResponseForwarding struct {
	// FlushInterval defines the interval, in milliseconds, in between flushes to the client while copying the response body.
	// A negative value means to flush immediately after each write to the client.
	// This configuration is ignored when ReverseProxy recognizes a response as a streaming response;
	// for such responses, writes are flushed to the client immediately.
	// Default: 100ms
	FlushInterval types.Duration `json:"flushInterval,omitempty" toml:"flushInterval,omitempty" yaml:"flushInterval,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// ServerHealthCheck holds the HealthCheck configuration.
type ServerHealthCheck struct {
	Scheme            string            `json:"scheme,omitempty" toml:"scheme,omitempty" yaml:"scheme,omitempty" export:"true"`
	Mode              string            `json:"mode,omitempty" toml:"mode,omitempty" yaml:"mode,omitempty" export:"true"`
	Path              string            `json:"path,omitempty" toml:"path,omitempty" yaml:"path,omitempty" export:"true"`
	Method            string            `json:"method,omitempty" toml:"method,omitempty" yaml:"method,omitempty" export:"true"`
	Status            int               `json:"status,omitempty" toml:"status,omitempty" yaml:"status,omitempty" export:"true"`
	Port              int               `json:"port,omitempty" toml:"port,omitempty,omitzero" yaml:"port,omitempty" export:"true"`
	Interval          types.Duration    `json:"interval,omitempty" toml:"interval,omitempty" yaml:"interval,omitempty" export:"true"`
	UnhealthyInterval *types.Duration   `json:"unhealthyInterval,omitempty" toml:"unhealthyInterval,omitempty" yaml:"unhealthyInterval,omitempty" export:"true"`
	Timeout           types.Duration    `json:"timeout,omitempty" toml:"timeout,omitempty" yaml:"timeout,omitempty" export:"true"`
	Hostname          string            `json:"hostname,omitempty" toml:"hostname,omitempty" yaml:"hostname,omitempty"`
	FollowRedirects   *bool             `json:"followRedirects,omitempty" toml:"followRedirects,omitempty" yaml:"followRedirects,omitempty" export:"true"`
	Headers           map[string]string `json:"headers,omitempty" toml:"headers,omitempty" yaml:"headers,omitempty" export:"true"`
}

// Sticky holds the sticky configuration.
type Sticky struct {
	// Cookie defines the sticky cookie configuration.
	Cookie *Cookie `json:"cookie,omitempty" toml:"cookie,omitempty" yaml:"cookie,omitempty" label:"allowEmpty" file:"allowEmpty" kv:"allowEmpty" export:"true"`
}

// +k8s:deepcopy-gen=true

// Cookie holds the sticky configuration based on cookie.
type Cookie struct {
	// Name defines the Cookie name.
	Name string `json:"name,omitempty" toml:"name,omitempty" yaml:"name,omitempty" export:"true"`
	// Secure defines whether the cookie can only be transmitted over an encrypted connection (i.e. HTTPS).
	Secure bool `json:"secure,omitempty" toml:"secure,omitempty" yaml:"secure,omitempty" export:"true"`
	// HTTPOnly defines whether the cookie can be accessed by client-side APIs, such as JavaScript.
	HTTPOnly bool `json:"httpOnly,omitempty" toml:"httpOnly,omitempty" yaml:"httpOnly,omitempty" export:"true"`
	// SameSite defines the same site policy.
	// More info: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie/SameSite
	// +kubebuilder:validation:Enum=none;lax;strict;None;Lax;Strict
	SameSite string `json:"sameSite,omitempty" toml:"sameSite,omitempty" yaml:"sameSite,omitempty" export:"true"`
	// MaxAge defines the number of seconds until the cookie expires.
	// When set to a negative number, the cookie expires immediately.
	// When set to zero, the cookie never expires.
	MaxAge int `json:"maxAge,omitempty" toml:"maxAge,omitempty" yaml:"maxAge,omitempty" export:"true"`
	// Path defines the path that must exist in the requested URL for the browser to send the Cookie header.
	// When not provided the cookie will be sent on every request to the domain.
	// More info: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#pathpath-value
	Path *string `json:"path,omitempty" toml:"path,omitempty" yaml:"path,omitempty" export:"true"`
	// Domain defines the host to which the cookie will be sent.
	// More info: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#domaindomain-value
	Domain string `json:"domain,omitempty" toml:"domain,omitempty" yaml:"domain,omitempty"`

	// Expires defines the number of seconds to add to the current time to calculate the expiration date of the cookie.
	// This option is exposed only for the Ingress NGINX provider.
	Expires int `json:"-" toml:"-" yaml:"-" label:"-" file:"-" kv:"-" export:"true"`
}

// +k8s:deepcopy-gen=true

// Server holds the server configuration.
type Server struct {
	URL          string `json:"url,omitempty" toml:"url,omitempty" yaml:"url,omitempty"`
	Weight       *int   `json:"weight,omitempty" toml:"weight,omitempty" yaml:"weight,omitempty" export:"true"`
	PreservePath bool   `json:"preservePath,omitempty" toml:"preservePath,omitempty" yaml:"preservePath,omitempty" export:"true"`
	Fenced       bool   `json:"fenced,omitempty" toml:"-" yaml:"-" label:"-" file:"-" kv:"-"`
	// Scheme can only be defined with label Providers.
	Scheme string `json:"-" toml:"-" yaml:"-" file:"-" kv:"-"`
	Port   string `json:"-" toml:"-" yaml:"-" file:"-" kv:"-"`
}
