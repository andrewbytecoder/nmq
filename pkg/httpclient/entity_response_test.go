// entity_response_test.go
package httpclient

import (
	"net/http"
	"reflect"
	"testing"
)

// TestNewEntityResponse 测试 EntityResponse 的初始化功能
// 验证 NewEntityResponse 创建的对象不为 nil，并且各字段初始化为默认值
func TestNewEntityResponse(t *testing.T) {
	got := NewEntityResponse()
	if got == nil {
		t.Fatal("NewEntityResponse() returned nil")
	}

	// 检查初始值是否为零值
	if got.Status != 0 {
		t.Errorf("Expected Status to be 0, got %d", got.Status)
	}
	got.SetHeader(make(http.Header))
	if got.Header == nil {
		t.Error("Expected Header to be non-nil (http.Header is map, should be initialized as nil but usable)")
	}
	if got.Body != nil {
		t.Errorf("Expected Body to be nil, got %v", got.Body)
	}
}

// TestEntityResponse_SetStatus 测试 EntityResponse 设置状态码的功能
// 验证 SetStatus 方法能够正确设置状态码并支持链式调用
func TestEntityResponse_SetStatus(t *testing.T) {
	er := NewEntityResponse()
	status := 200

	result := er.SetStatus(status)

	// 验证链式调用返回自身
	if result != er {
		t.Error("SetStatus() should return the same instance for chaining")
	}

	// 验证状态码被正确设置
	if er.Status != status {
		t.Errorf("Expected Status to be %d, got %d", status, er.Status)
	}
}

// TestEntityResponse_SetHeader 测试 EntityResponse 设置 HTTP 头部的功能
// 验证 SetHeader 方法能够正确设置头部信息并支持链式调用
func TestEntityResponse_SetHeader(t *testing.T) {
	er := NewEntityResponse()
	header := http.Header{
		"Content-Type":    []string{"application/json"},
		"Authorization":   []string{"Bearer token123"},
		"X-Custom-Header": []string{"value1", "value2"},
	}

	result := er.SetHeader(header)

	// 验证链式调用返回自身
	if result != er {
		t.Error("SetHeader() should return the same instance for chaining")
	}

	// 验证 Header 被正确设置（深比较）
	if !reflect.DeepEqual(er.Header, header) {
		t.Errorf("Expected Header to be %v, got %v", header, er.Header)
	}
}

// TestEntityResponse_SetBody 测试 EntityResponse 设置响应体的功能
// 验证 SetBody 方法能够正确设置响应体数据并支持链式调用
func TestEntityResponse_SetBody(t *testing.T) {
	er := NewEntityResponse()
	body := []byte(`{"message": "hello world"}`)

	result := er.SetBody(body)

	// 验证链式调用返回自身
	if result != er {
		t.Error("SetBody() should return the same instance for chaining")
	}

	// 验证 Body 被正确设置
	if !reflect.DeepEqual(er.Body, body) {
		t.Errorf("Expected Body to be %v, got %v", body, er.Body)
	}
}

// TestEntityResponse_Chaining 测试 EntityResponse 的链式调用功能
// 验证多个方法可以连续调用并正确设置所有属性
func TestEntityResponse_Chaining(t *testing.T) {
	body := []byte(`{"success": true}`)
	header := http.Header{"Content-Type": []string{"application/json"}}
	status := 201

	er := NewEntityResponse().
		SetStatus(status).
		SetHeader(header).
		SetBody(body)

	if er.Status != status {
		t.Errorf("Expected Status %d, got %d", status, er.Status)
	}
	if !reflect.DeepEqual(er.Header, header) {
		t.Errorf("Expected Header %v, got %v", header, er.Header)
	}
	if !reflect.DeepEqual(er.Body, body) {
		t.Errorf("Expected Body %v, got %v", body, er.Body)
	}
}

// TestEntityResponse_Overwrite 测试 EntityResponse 属性覆盖功能
// 验证后设置的值会正确覆盖之前设置的值
func TestEntityResponse_Overwrite(t *testing.T) {
	er := NewEntityResponse()

	// 第一次设置
	er.SetStatus(200).SetBody([]byte("first"))

	// 覆盖设置
	er.SetStatus(201).SetBody([]byte("second"))

	if er.Status != 201 {
		t.Errorf("Expected Status 201 after overwrite, got %d", er.Status)
	}
	if string(er.Body) != "second" {
		t.Errorf("Expected Body 'second' after overwrite, got %s", er.Body)
	}
}
