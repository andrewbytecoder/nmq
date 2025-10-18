package check

import (
	"testing"
)

func TestIsValidPyroscopeAddressSimple(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{
			name:     "valid domain with port",
			addr:     "http://pyroscope-server:4040",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPyroscopeAddress(tt.addr)
			if got != tt.expected {
				t.Errorf("IsValidPyroscopeAddress(%q) = %v, want %v", tt.addr, got, tt.expected)
			}
		})
	}
}

func TestIsValidPyroscopeAddress(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		// ✅ 合法用例
		{
			name:     "valid domain with port",
			addr:     "http://pyroscope-server:4040",
			expected: true,
		},
		{
			name:     "valid localhost with port",
			addr:     "http://localhost:8080",
			expected: true,
		},
		{
			name:     "valid IPv4 with port",
			addr:     "http://192.168.1.100:4040",
			expected: true,
		},
		{
			name:     "valid IPv6 with port",
			addr:     "http://[::1]:4040",
			expected: true,
		},
		{
			name:     "valid example.com with port 80",
			addr:     "http://example.com:80",
			expected: true,
		},
		{
			name:     "valid with trailing slash",
			addr:     "http://pyroscope-server:4040/",
			expected: true,
		},

		// ❌ 非法用例 - scheme 错误
		{
			name:     "https scheme not allowed",
			addr:     "https://pyroscope-server:4040",
			expected: false,
		},
		{
			name:     "no scheme",
			addr:     "pyroscope-server:4040",
			expected: false,
		},
		{
			name:     "ftp scheme",
			addr:     "ftp://pyroscope-server:4040",
			expected: false,
		},

		// ❌ 非法用例 - 缺少端口
		{
			name:     "missing port",
			addr:     "http://pyroscope-server",
			expected: false,
		},
		{
			name:     "IPv4 without port",
			addr:     "http://192.168.1.100",
			expected: false,
		},
		{
			name:     "IPv6 without port",
			addr:     "http://[::1]",
			expected: false,
		},

		// ❌ 非法用例 - 端口无效
		{
			name:     "empty port",
			addr:     "http://pyroscope-server:",
			expected: false,
		},
		{
			name:     "non-numeric port",
			addr:     "http://pyroscope-server:abc",
			expected: false,
		},
		{
			name:     "port zero",
			addr:     "http://pyroscope-server:0",
			expected: false,
		},
		{
			name:     "port too large",
			addr:     "http://pyroscope-server:70000",
			expected: false,
		},

		// ❌ 非法用例 - 路径/查询/fragment
		{
			name:     "has path",
			addr:     "http://pyroscope-server:4040/metrics",
			expected: false,
		},
		{
			name:     "has query",
			addr:     "http://pyroscope-server:4040?debug=true",
			expected: false,
		},
		{
			name:     "has fragment",
			addr:     "http://pyroscope-server:4040#section",
			expected: false,
		},
		{
			name:     "has path and query",
			addr:     "http://pyroscope-server:4040/api?debug=1",
			expected: false,
		},

		// ❌ 非法用例 - 格式错误
		{
			name:     "empty string",
			addr:     "",
			expected: false,
		},
		{
			name:     "only http",
			addr:     "http://",
			expected: false,
		},
		{
			name:     "missing host",
			addr:     "http://:4040",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPyroscopeAddress(tt.addr)
			if got != tt.expected {
				t.Errorf("IsValidPyroscopeAddress(%q) = %v, want %v", tt.addr, got, tt.expected)
			}
		})
	}
}
