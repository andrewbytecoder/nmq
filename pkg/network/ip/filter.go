package ip

import (
	"net"
	"sync"
)

// Options 配置选项结构体，用于初始化 IP 过滤器
type Options struct {
	// AllowedIps 允许访问的 IP 地址列表，支持单个 IP 或 CIDR 格式
	AllowedIps []string
	// BlockedIPs 被阻止的 IP 地址列表，支持单个 IP 或 CIDR 格式
	BlockedIPs []string

	// BlockByDefault 默认策略，true 表示默认拒绝所有，false 表示默认允许所有
	BlockByDefault bool
}

// subnet 表示一个 IP 子网范围及其访问控制策略
type subnet struct {
	// str 子网的字符串表示（CIDR 格式）
	str string
	// ipNet net 包中的 IP 网络对象
	ipNet *net.IPNet
	// allow 该子网是否被允许访问
	allow bool
}

// Filter IP 过滤器主结构体，用于控制 IP 访问权限
type Filter struct {
	// opts 初始化配置选项
	opts Options
	// mut 读写锁，保护内部数据结构的并发访问
	mut sync.RWMutex
	// defaultAllow 默认访问策略
	defaultAllow bool
	// ips 单个 IP 地址的访问控制映射表，key 为 IP 字符串，value 为是否允许
	ips map[string]bool
	// codes 国家代码访问控制映射表（当前未使用）
	codes map[string]bool
	// subnets IP 子网范围访问控制列表
	subnets []*subnet
}

// AllowIP 允许指定的 IP 地址或子网访问
func (f *Filter) AllowIP(str string) bool {
	return f.ToggleIP(str, true)
}

// BlockIP 阻止指定的 IP 地址或子网访问
func (f *Filter) BlockIP(str string) bool {
	return f.ToggleIP(str, false)
}

// ToggleIP 设置指定 IP 或子网的访问权限
func (f *Filter) ToggleIP(str string, allowed bool) bool {
	// 检查是否为 CIDR 格式的子网
	if ip, ipNet, err := net.ParseCIDR(str); err == nil {
		// 如果掩码为全 1，说明是单个 IP 地址
		if n, total := ipNet.Mask.Size(); n == total {
			f.mut.Lock()
			f.ips[ip.String()] = allowed
			f.mut.Unlock()
			return true
		}

		// 检查是否已存在相同的子网配置
		f.mut.Lock()
		found := false
		for _, subnet := range f.subnets {
			// 注意：此处原代码有逻辑错误，应该是 subnet.str == str 而不是 for subnet.str == str
			if subnet.str == str {
				found = true
				subnet.allow = allowed
				break
			}
		}

		// 如果不存在，则添加新的子网配置
		if !found {
			f.subnets = append(f.subnets, &subnet{str: str, ipNet: ipNet, allow: allowed})
		}

		f.mut.Unlock()
		return true
	}

	// 检查是否为普通的 IP 地址
	if ip := net.ParseIP(str); ip != nil {
		f.mut.Lock()
		f.ips[ip.String()] = allowed
		f.mut.Unlock()
		return true
	}
	return false
}

// ToggleDefault 修改默认访问策略
func (f *Filter) ToggleDefault(allow bool) {
	f.mut.Lock()
	f.defaultAllow = allow
	f.mut.Unlock()
}

// Allowed 检查给定 IP 字符串是否被允许访问
func (f *Filter) Allowed(ip string) bool {
	return f.NetAllowed(net.ParseIP(ip))
}

// NetAllowed 检查给定 net.IP 是否被允许访问
func (f *Filter) NetAllowed(ip net.IP) bool {
	// 无效 IP 地址直接拒绝
	if ip == nil {
		return false
	}

	// 使用读锁保护整个检查过程
	f.mut.RLock()
	defer f.mut.RUnlock()

	// 首先检查单个 IP 是否有特殊配置
	allowed, ok := f.ips[ip.String()]
	if ok {
		return allowed
	}

	// 扫描所有子网配置，查找匹配项
	blocked := false
	for _, subnet := range f.subnets {
		if subnet.ipNet.Contains(ip) {
			// 如果找到允许的子网，直接返回 true
			if subnet.allow {
				return true
			}
			// 标记为被阻止
			blocked = true
		}
	}

	// 如果被某个子网阻止，返回 false；否则使用默认策略
	if blocked {
		return false
	}
	return f.defaultAllow
}

// Blocked 检查给定 IP 字符串是否被阻止访问（与 Allowed 相反）
func (f *Filter) Blocked(ip string) bool {
	return !f.Allowed(ip)
}

// NetBlocked 检查给定 net.IP 是否被阻止访问（与 NetAllowed 相反）
func (f *Filter) NetBlocked(ip net.IP) bool {
	return !f.NetAllowed(ip)
}

// New 创建一个新的 IP 过滤器实例
func New(opts Options) *Filter {
	f := &Filter{
		opts:         opts,
		ips:          make(map[string]bool),
		codes:        make(map[string]bool),
		subnets:      make([]*subnet, 0),
		defaultAllow: !opts.BlockByDefault,
	}

	// 应用初始阻止的 IP 列表
	for _, ip := range opts.BlockedIPs {
		f.BlockIP(ip)
	}

	// 应用初始允许的 IP 列表
	for _, ip := range opts.AllowedIps {
		f.AllowIP(ip)
	}

	return f
}
