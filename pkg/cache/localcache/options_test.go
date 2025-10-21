// option_test.go
package localcache

import (
	"testing"

	"hytera.com/ncp/pkg/options"
)

func TestConfigStruct(t *testing.T) {
	// 测试 Config 结构体初始化
	config := &Config{
		capture: func(key string, value interface{}) {
			// 测试捕获函数
		},
		member: make(map[string]Iterator),
	}

	if config.capture == nil {
		t.Error("Expected capture function to be initialized")
	}

	if config.member == nil {
		t.Error("Expected member map to be initialized")
	}
}

func TestSetCapture(t *testing.T) {
	// 测试 SetCapture 选项函数
	var capturedKey string
	var capturedValue interface{}

	captureFunc := func(key string, value interface{}) {
		capturedKey = key
		capturedValue = value
	}

	option := SetCapture(captureFunc)

	config := &Config{}
	option(config)

	// 验证 capture 函数被正确设置
	if config.capture == nil {
		t.Error("Expected capture function to be set")
	}

	// 验证 capture 函数可以被调用
	testKey := "test_key"
	testValue := "test_value"
	config.capture(testKey, testValue)

	if capturedKey != testKey {
		t.Errorf("Expected captured key to be %s, got %s", testKey, capturedKey)
	}

	if capturedValue != testValue {
		t.Errorf("Expected captured value to be %s, got %v", testValue, capturedValue)
	}
}

func TestSetMember(t *testing.T) {
	// 测试 SetMember 选项函数
	memberMap := map[string]Iterator{
		"key1": {Val: "value1", Expire: 0},
		"key2": {Val: "value2", Expire: 0},
	}

	option := SetMember(memberMap)

	config := &Config{}
	option(config)

	// 验证 member map 被正确设置
	if len(config.member) != len(memberMap) {
		t.Errorf("Expected member map length to be %d, got %d", len(memberMap), len(config.member))
	}

	for k, v := range memberMap {
		if config.member[k].Val != v.Val {
			t.Errorf("Expected member[%s] value to be %v, got %v", k, v.Val, config.member[k].Val)
		}
	}
}

func TestNewConfig(t *testing.T) {
	// 测试 NewConfig 函数基本功能
	config := NewConfig()

	if config == nil {
		t.Error("Expected config to be created")
	}

	if config.capture == nil {
		t.Error("Expected default capture function to be set")
	}
}

func TestNewConfigWithOptions(t *testing.T) {
	// 测试 NewConfig 函数与选项的集成
	var capturedKey string
	captureFunc := func(key string, value interface{}) {
		capturedKey = key
	}

	memberMap := map[string]Iterator{
		"test": {Val: "value", Expire: 0},
	}

	config := NewConfig(
		SetCapture(captureFunc),
		SetMember(memberMap),
	)

	// 验证选项被正确应用
	if len(config.member) != 1 {
		t.Errorf("Expected member map length to be 1, got %d", len(config.member))
	}

	if config.member["test"].Val != "value" {
		t.Errorf("Expected member[test] value to be 'value', got %v", config.member["test"].Val)
	}

	// 验证自定义 capture 函数被应用
	testKey := "test_key"
	config.capture(testKey, nil)
	if capturedKey != testKey {
		t.Errorf("Expected captured key to be %s, got %s", testKey, capturedKey)
	}
}

func TestOptionFunctionType(t *testing.T) {
	// 测试选项函数符合 options.Option 类型
	var _ options.Option = SetCapture(nil)
	var _ options.Option = SetMember(nil)
}
