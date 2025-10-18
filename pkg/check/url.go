package check

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

// IsValidHTTPAddress 检查 addr 是否为合法的 http:// 或 https:// 地址
// 要求：必须有 scheme（http/https）、host（域名或IP）、可选端口，不能有路径、查询参数或 fragment
func IsValidHTTPAddress(addr string) bool {
	if addr == "" {
		return false
	}

	u, err := url.Parse(addr)
	if err != nil {
		return false
	}

	// 必须是 http 或 https
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// 必须有 host（包含域名或IP，可带端口）
	if u.Host == "" {
		return false
	}

	// 不允许有路径（除了根路径 "/"）、查询参数、fragment
	// 注意：url.Parse("http://a:4040") 的 u.Path 是 ""，这是合法的
	//       url.Parse("http://a:4040/") 的 u.Path 是 "/"，通常也可接受
	if u.Path != "" && u.Path != "/" {
		return false
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	// 可选：进一步校验 host 格式（如是否包含非法字符）
	// 但 url.Parse 已做基本校验，通常够用

	return true
}

// IsValidPyroscopeAddress 严格校验地址是否为 http://host:port 格式
func IsValidPyroscopeAddress(addr string) bool {
	if addr == "" {
		return false
	}

	u, err := url.Parse(addr)
	if err != nil {
		return false
	}

	// 1. 必须是 http（不允许 https）
	if u.Scheme != "http" {
		return false
	}

	// 2. 不能有路径、查询、fragment
	if u.Path != "" && u.Path != "/" {
		return false
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	// 3. host 必须存在且包含端口
	host := u.Host
	if host == "" {
		return false
	}

	// 分离 host 和 port
	var hostname, port string
	if strings.HasPrefix(host, "[") {
		// IPv6 格式: [::1]:4040
		end := strings.LastIndex(host, "]")
		if end == -1 {
			return false // 无效 IPv6
		}
		hostname = host[:end+1]
		if len(host) > end+1 {
			if host[end+1] != ':' {
				return false
			}
			port = host[end+2:]
		} else {
			return false // 没有端口
		}
	} else {
		// IPv4 或域名: port
		parts := strings.Split(host, ":")
		if len(parts) < 2 {
			return false // 没有端口
		}
		// 处理 IPv4（可能含多个冒号？不，IPv4 无冒号），所以最后一部分是 port
		port = parts[len(parts)-1]
		hostname = strings.Join(parts[:len(parts)-1], ":")
	}

	// 4. 端口不能为空
	if port == "" {
		return false
	}

	// 5. 端口必须是 1~65535 的整数
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return false
	}

	// 6. hostname 不能为空（如 ":4040" 非法）
	if hostname == "" {
		return false
	}

	// 7. （可选）校验 hostname 是否为合法 IP 或域名
	// 这里我们只做基本检查：不是纯数字（避免 1234 被当 host）
	// 更严格的域名校验可用 regexp，但通常 url.Parse 已足够
	if net.ParseIP(strings.Trim(hostname, "[]")) == nil {
		// 不是 IP，当作域名处理：简单检查是否含非法字符
		// 实际中可跳过，或使用更复杂的校验
		// 此处我们信任 url.Parse 的 host 解析
	}

	return true
}
