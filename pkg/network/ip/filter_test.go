package ip

import (
	"net"
	"testing"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name           string
		opts           Options
		expectedFilter *Filter
	}{
		{
			name: "默认允许策略",
			opts: Options{
				BlockByDefault: false,
			},
			expectedFilter: &Filter{
				defaultAllow: true,
			},
		},
		{
			name: "默认阻止策略",
			opts: Options{
				BlockByDefault: true,
			},
			expectedFilter: &Filter{
				defaultAllow: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := New(tt.opts)
			if filter.defaultAllow != tt.expectedFilter.defaultAllow {
				t.Errorf("New() defaultAllow = %v, want %v", filter.defaultAllow, tt.expectedFilter.defaultAllow)
			}
		})
	}
}

func TestToggleIP(t *testing.T) {
	tests := []struct {
		name     string
		ipStr    string
		allowed  bool
		expected bool
	}{
		{
			name:     "有效的单个IP地址",
			ipStr:    "192.168.1.1",
			allowed:  true,
			expected: true,
		},
		{
			name:     "有效的CIDR子网",
			ipStr:    "192.168.1.0/24",
			allowed:  false,
			expected: true,
		},
		{
			name:     "单个IP的CIDR表示",
			ipStr:    "192.168.1.1/32",
			allowed:  true,
			expected: true,
		},
		{
			name:     "无效的IP地址",
			ipStr:    "invalid-ip",
			allowed:  true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := New(Options{})
			result := filter.ToggleIP(tt.ipStr, tt.allowed)
			if result != tt.expected {
				t.Errorf("ToggleIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAllowAndBlockIP(t *testing.T) {
	filter := New(Options{})

	// 测试允许IP
	result := filter.AllowIP("192.168.1.1")
	if !result {
		t.Error("AllowIP() should return true for valid IP")
	}

	// 测试阻止IP
	result = filter.BlockIP("192.168.1.2")
	if !result {
		t.Error("BlockIP() should return true for valid IP")
	}
}

func TestAllowedAndBlocked(t *testing.T) {
	tests := []struct {
		name        string
		opts        Options
		testIP      string
		shouldAllow bool
	}{
		{
			name: "默认允许策略下的普通IP",
			opts: Options{
				BlockByDefault: false,
			},
			testIP:      "192.168.1.1",
			shouldAllow: true,
		},
		{
			name: "默认阻止策略下的普通IP",
			opts: Options{
				BlockByDefault: true,
			},
			testIP:      "192.168.1.1",
			shouldAllow: false,
		},
		{
			name: "明确允许的IP",
			opts: Options{
				AllowedIps:     []string{"192.168.1.1"},
				BlockByDefault: true,
			},
			testIP:      "192.168.1.1",
			shouldAllow: true,
		},
		{
			name: "明确阻止的IP",
			opts: Options{
				BlockedIPs:     []string{"192.168.1.1"},
				BlockByDefault: false,
			},
			testIP:      "192.168.1.1",
			shouldAllow: false,
		},
		{
			name: "子网内允许的IP",
			opts: Options{
				AllowedIps:     []string{"192.168.1.0/24"},
				BlockByDefault: true,
			},
			testIP:      "192.168.1.100",
			shouldAllow: true,
		},
		{
			name: "子网内阻止的IP",
			opts: Options{
				BlockedIPs:     []string{"192.168.1.0/24"},
				BlockByDefault: false,
			},
			testIP:      "192.168.1.100",
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := New(tt.opts)
			allowed := filter.Allowed(tt.testIP)
			blocked := filter.Blocked(tt.testIP)

			if allowed != tt.shouldAllow {
				t.Errorf("Allowed() = %v, want %v for IP %s", allowed, tt.shouldAllow, tt.testIP)
			}

			if blocked == tt.shouldAllow {
				t.Errorf("Blocked() = %v, want %v for IP %s", blocked, !tt.shouldAllow, tt.testIP)
			}
		})
	}
}

func TestToggleDefault(t *testing.T) {
	filter := New(Options{BlockByDefault: false})

	if !filter.defaultAllow {
		t.Error("Expected defaultAllow to be true initially")
	}

	filter.ToggleDefault(false)
	if filter.defaultAllow {
		t.Error("Expected defaultAllow to be false after ToggleDefault(false)")
	}

	filter.ToggleDefault(true)
	if !filter.defaultAllow {
		t.Error("Expected defaultAllow to be true after ToggleDefault(true)")
	}
}

func TestNetAllowedAndNetBlocked(t *testing.T) {
	filter := New(Options{})

	// 测试无效IP
	invalidIP := net.ParseIP("")
	if filter.NetAllowed(invalidIP) {
		t.Error("NetAllowed() should return false for invalid IP")
	}

	if !filter.NetBlocked(invalidIP) {
		t.Error("NetBlocked() should return true for invalid IP")
	}

	// 测试有效IP
	validIP := net.ParseIP("192.168.1.1")
	if !filter.NetAllowed(validIP) {
		t.Error("NetAllowed() should return true for valid IP with default allow policy")
	}

	if filter.NetBlocked(validIP) {
		t.Error("NetBlocked() should return false for valid IP with default allow policy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	filter := New(Options{})

	// 并发测试多个goroutine同时访问
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			filter.AllowIP("192.168.1.1")
			filter.BlockIP("192.168.1.2")
			filter.Allowed("192.168.1.1")
			filter.Blocked("192.168.1.2")
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
