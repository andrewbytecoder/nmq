package http

import (
	"errors"
	"math"
	"net"
	"net/http"
	"strings"
)

// xForwardedFor 定义了 HTTP 请求头 X-Forwarded-For 的名称，用于获取客户端真实 IP 地址
const xForwardedFor = "X-Forwarded-For"

// xRealIP 定义了 HTTP 请求头 X-Real-IP 的名称，也用于获取客户端真实 IP 地址
const xRealIP = "X-Real-IP"

// HasLocalIPAddr 检查给定的 IP 字符串是否为本地地址（回环、链路本地单播或多播）
func HasLocalIPAddr(ip string) bool {
	return HasLocalIp(net.ParseIP(ip))
}

// HasLocalIp 判断一个 net.IP 是否是本地地址
func HasLocalIp(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return false
}

// ClientIP 获取客户端的真实 IP 地址，优先从 X-Forwarded-For 或 X-Real-IP 头部获取，
// 如果没有则从请求的远程地址中提取
func ClientIP(req *http.Request) string {
	if ip := strings.TrimSpace(strings.Split(req.Header.Get(xForwardedFor), ",")[0]); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(req.Header.Get(xRealIP)); ip != "" {
		return ip
	}
	return RemoteIP(req)
}

// ClientPublicIP 获取客户端的公网 IP 地址，排除本地地址
func ClientPublicIP(req *http.Request) string {
	if ip := strings.TrimSpace(strings.Split(req.Header.Get(xForwardedFor), ",")[0]); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	if ip := strings.TrimSpace(req.Header.Get(xRealIP)); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	if ip := RemoteIP(req); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	return ""
}

// RemoteIP 从 http.Request 中提取远程主机的 IP 地址
func RemoteIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		return ""
	}
	return ip
}

// IsWebsocket 判断当前请求是否为 WebSocket 连接升级请求
func IsWebsocket(req *http.Request) bool {
	if strings.Contains(strings.ToLower(requestHeader(req, "Connection")), "upgrade") &&
		strings.EqualFold(requestHeader(req, "Upgrade"), "websocket") {
		return true
	}
	return false
}

// requestHeader 获取指定键的 HTTP 请求头部值
func requestHeader(req *http.Request, key string) string {
	if req == nil {
		return ""
	}
	return req.Header.Get(key)
}

// ContentType 返回请求的 Content-Type 头部，并过滤掉参数部分
func ContentType(req *http.Request) string {
	return filterFlags(requestHeader(req, "Content-Type"))
}

// filterFlags 移除字符串中的标志位（空格或分号后的内容）
func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

// StringToLong 将 IPv4 地址字符串转换为无符号整数表示形式
func StringToLong(ip string) (uint, error) {
	b := net.ParseIP(ip).To4()
	if b == nil {
		return 0, errors.New("invalid ipv4 format")
	}

	return uint(b[3]) | uint(b[2])<<8 | uint(b[1])<<16 | uint(b[0])<<24, nil
}

// LongToIPString 将无符号整数表示的 IPv4 地址转换为标准的点分十进制字符串格式
func LongToIPString(i uint) (string, error) {
	if i > math.MaxUint32 {
		return "", errors.New("beyond the scope of ipv4")
	}

	ip := make(net.IP, net.IPv4len)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)
	return ip.String(), nil
}

// ToLong 将 net.IP 类型的 IPv4 地址转换为无符号整数表示形式
func ToLong(ip net.IP) (uint, error) {
	b := ip.To4()
	if b == nil {
		return 0, errors.New("invalid ipv4 format")
	}
	return uint(b[3]) | uint(b[2])<<8 | uint(b[1])<<16 | uint(b[0])<<24, nil
}

// LongToIP 将无符号整数表示的 IPv4 地址转换为 net.IP 类型
func LongToIP(i uint) (net.IP, error) {
	if i > math.MaxUint32 {
		return nil, errors.New("beyond the scope of ipv4")
	}

	ip := make(net.IP, net.IPv4len)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)
	return ip, nil
}
