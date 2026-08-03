package dynamic

// +k8s:deepcopy-gen=true

// UDPRouter defines the configuration for an UDP router.
type UDPRouter struct {
	EntryPoints []string `json:"entryPoints,omitempty" toml:"entryPoints,omitempty" yaml:"entryPoints,omitempty" export:"true"`
	Service     string   `json:"service,omitempty" toml:"service,omitempty" yaml:"service,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// UDPService defines the configuration for a UDP service. All fields are mutually exclusive.
type UDPService struct {
	// 直接指定后端地址列表
	LoadBalancer *UDPServersLoadBalancer `json:"loadBalancer,omitempty" toml:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty" export:"true"`
	// 引用其他service并按照权重进行加权处理
	Weighted *UDPWeightedRoundRobin `json:"weighted,omitempty" toml:"weighted,omitempty" yaml:"weighted,omitempty" label:"-" export:"true"`
}

// +k8s:deepcopy-gen=true

// UDPServer defines a UDP server configuration.
// port字段不会参与序列化和反序列化
type UDPServer struct {
	Address string `json:"address,omitempty" toml:"address,omitempty" yaml:"address,omitempty" label:"-"`
	Port    string `json:"-" toml:"-" yaml:"-" file:"-"`
}

// +k8s:deepcopy-gen=true

// UDPWeightedRoundRobin is a weighted round robin UDP load-balancer of services.
type UDPWeightedRoundRobin struct {
	Services []UDPWRRService `json:"services,omitempty" toml:"services,omitempty" yaml:"services,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// UDPWRRService is a reference to a UDP service load-balanced with weighted round robin.
type UDPWRRService struct {
	Name   string `json:"name,omitempty" toml:"name,omitempty" yaml:"name,omitempty" export:"true"`
	Weight *int   `json:"weight,omitempty" toml:"weight,omitempty" yaml:"weight,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// UDPServersLoadBalancer defines the configuration for a load-balancer of UDP servers.
type UDPServersLoadBalancer struct {
	Servers []UDPServer `json:"servers,omitempty" toml:"servers,omitempty" yaml:"servers,omitempty" label-slice-as-struct:"server" export:"true"`
}
