// err_test.go
package localcache

import (
	"errors"
	"testing"
)

func TestCacheErrors(t *testing.T) {
	// 测试各种错误类型的创建和值
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"CacheExist", CacheExist, "local_cache: cache exist"},
		{"CacheNoExist", CacheNoExist, "local_cache: cache no exist"},
		{"CacheExpire", CacheExpire, "local_cache: cache expire"},
		{"CacheTypeErr", CacheTypeErr, "local_cache: cache incr type err"},
		{"CacheGobErr", CacheGobErr, "local_cache: cache save gob err"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %s, got %s", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestCacheErrExist(t *testing.T) {
	// 测试 CacheErrExist 函数
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheExist error", CacheExist, true},
		{"Other error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheErrExist(tt.err)
			if result != tt.expected {
				t.Errorf("CacheErrExist(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCacheErrNoExist(t *testing.T) {
	// 测试 CacheErrNoExist 函数
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheNoExist error", CacheNoExist, true},
		{"Other error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheErrNoExist(tt.err)
			if result != tt.expected {
				t.Errorf("CacheErrNoExist(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCacheErrExpire(t *testing.T) {
	// 测试 CacheErrExpire 函数
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheExpire error", CacheExpire, true},
		{"Other error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheErrExpire(tt.err)
			if result != tt.expected {
				t.Errorf("CacheErrExpire(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCacheErrTypeErr(t *testing.T) {
	// 测试 CacheErrTypeErr 函数
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheTypeErr error", CacheTypeErr, true},
		{"Other error", errors.New("other error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheErrTypeErr(tt.err)
			if result != tt.expected {
				t.Errorf("CacheErrTypeErr(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	// 测试错误包装情况下的判断
	wrappedExistErr := errors.New("wrapped error")
	wrappedExistErr = errors.Join(wrappedExistErr, CacheExist)

	if !CacheErrExist(wrappedExistErr) {
		t.Error("CacheErrExist should return true for wrapped CacheExist error")
	}
}
