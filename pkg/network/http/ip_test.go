package http

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestHasLocalIPAddr(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"localhost", "127.0.0.1", true},
		{"loopback", "127.0.0.2", true},
		{"link local unicast", "169.254.0.1", true},
		{"public ip", "8.8.8.8", false},
		{"private ip", "192.168.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasLocalIPAddr(tt.ip); got != tt.want {
				t.Errorf("HasLocalIPAddr(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "X-Forwarded-For present",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1, 10.0.0.1",
			},
			remoteAddr: "127.0.0.1:8080",
			expectedIP: "192.168.1.1",
		},
		{
			name: "X-Real-IP present",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.2",
			},
			remoteAddr: "127.0.0.1:8080",
			expectedIP: "192.168.1.2",
		},
		{
			name:       "No headers, use remote addr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.3:8080",
			expectedIP: "192.168.1.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			if got := ClientIP(req); got != tt.expectedIP {
				t.Errorf("ClientIP() = %v, want %v", got, tt.expectedIP)
			}
		})
	}
}

func TestClientPublicIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "X-Forwarded-For with public IP",
			headers: map[string]string{
				"X-Forwarded-For": "8.8.8.8, 10.0.0.1",
			},
			remoteAddr: "127.0.0.1:8080",
			expectedIP: "8.8.8.8",
		},
		{
			name: "X-Forwarded-For with private IP, fallback to X-Real-IP",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-IP":       "8.8.8.8",
			},
			remoteAddr: "127.0.0.1:8080",
			expectedIP: "192.168.1.1",
		},
		{
			name: "All IPs are local, return empty",
			headers: map[string]string{
				"X-Forwarded-For": "127.0.0.1",
				"X-Real-IP":       "192.168.1.1",
			},
			remoteAddr: "10.0.0.1:8080",
			expectedIP: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			if got := ClientPublicIP(req); got != tt.expectedIP {
				t.Errorf("ClientPublicIP() = %v, want %v", got, tt.expectedIP)
			}
		})
	}
}

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expectedIP string
	}{
		{"valid address", "192.168.1.1:8080", "192.168.1.1"},
		{"invalid address", "invalid", ""},
		{"IPv6 address", "[::1]:8080", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			if got := RemoteIP(req); got != tt.expectedIP {
				t.Errorf("RemoteIP() = %v, want %v", got, tt.expectedIP)
			}
		})
	}
}

func TestIsWebsocket(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name: "WebSocket upgrade request",
			headers: map[string]string{
				"Connection": "Upgrade",
				"Upgrade":    "websocket",
			},
			want: true,
		},
		{
			name: "Case insensitive WebSocket",
			headers: map[string]string{
				"Connection": "upgrade",
				"Upgrade":    "WEBSOCKET",
			},
			want: true,
		},
		{
			name: "Not a WebSocket request",
			headers: map[string]string{
				"Connection": "keep-alive",
				"Upgrade":    "other",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			if got := IsWebsocket(req); got != tt.want {
				t.Errorf("IsWebsocket() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{"simple type", "application/json", "application/json"},
		{"type with charset", "text/html; charset=utf-8", "text/html"},
		{"type with spaces", "application/xml ; charset=utf-8", "application/xml"},
		{"empty type", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.Header.Set("Content-Type", tt.contentType)

			if got := ContentType(req); got != tt.expected {
				t.Errorf("ContentType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIPConversion(t *testing.T) {
	tests := []struct {
		name      string
		ipString  string
		ipLong    uint
		expectErr bool
	}{
		{"valid IPv4", "192.168.1.1", 3232235777, false},
		{"localhost", "127.0.0.1", 2130706433, false},
		{"invalid IPv6", "::1", 0, true},
		{"invalid format", "not.an.ip", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 测试 StringToLong
			got, err := StringToLong(tt.ipString)
			if (err != nil) != tt.expectErr {
				t.Errorf("StringToLong() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && got != tt.ipLong {
				t.Errorf("StringToLong() = %v, want %v", got, tt.ipLong)
			}

			// 测试 LongToIPString (跳过错误情况)
			if !tt.expectErr {
				gotStr, err := LongToIPString(tt.ipLong)
				if err != nil {
					t.Errorf("LongToIPString() error = %v", err)
					return
				}
				if gotStr != tt.ipString {
					t.Errorf("LongToIPString() = %v, want %v", gotStr, tt.ipString)
				}
			}
		})
	}
}

func TestToLongAndLongToIP(t *testing.T) {
	ipStr := "192.168.1.1"
	ip := net.ParseIP(ipStr)

	// 测试 ToLong
	long, err := ToLong(ip)
	if err != nil {
		t.Errorf("ToLong() error = %v", err)
		return
	}

	expectedLong := uint(3232235777)
	if long != expectedLong {
		t.Errorf("ToLong() = %v, want %v", long, expectedLong)
	}

	// 测试 LongToIP
	ipBack, err := LongToIP(long)
	if err != nil {
		t.Errorf("LongToIP() error = %v", err)
		return
	}

	if ipBack.String() != "192.168.1.1" {
		t.Errorf("LongToIP() = %v, want %v", ipBack.String(), ipStr)
	}
}
