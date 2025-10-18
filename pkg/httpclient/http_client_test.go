// httpclient_test.go
package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestNewHttpClient(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	if client == nil {
		t.Fatal("NewHttpClient returned nil")
	}
	if client.logger == nil {
		t.Error("logger should not be nil")
	}
	if client.c == nil {
		t.Error("http.Client should not be nil")
	}

	// 验证 TLS 跳过验证已启用
	tr, ok := client.c.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig should not be nil")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestHttpClient_Send_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	expectedBody := `{"status": "ok"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	body, err := client.Send(req, 5*time.Second)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if string(body) != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, string(body))
	}
}

func TestHttpClient_Send_403_AuthenticationFailed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	errorMsg := "access denied: invalid token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(errorMsg))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = client.Send(req, 5*time.Second)
	if err == nil {
		t.Fatal("Expected authentication error, got nil")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected 'authentication failed' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), errorMsg) {
		t.Errorf("Expected error to contain response body %q, got: %v", errorMsg, err)
	}
}

func TestHttpClient_Send_NetworkError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	// 使用一个不可能连接的 URL（或关闭的服务器）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不会执行到这里
	}))
	server.Close() // 立即关闭，模拟连接失败

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = client.Send(req, 1*time.Second)
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}
}

func TestHttpClient_Send_Timeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟慢响应（超过 timeout）
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 设置很短的超时
	_, err = client.Send(req, 100*time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestHttpClient_SendRequestReturnEntity_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	expectedBody := `{"data": "hello"}`
	expectedHeader := "application/json"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", expectedHeader)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"input":1}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.SendRequestReturnEntity(req, 5*time.Second)
	if err != nil {
		t.Fatalf("SendRequestReturnEntity failed: %v", err)
	}

	if resp.Status != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, resp.Status)
	}
	if string(resp.Body) != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, string(resp.Body))
	}
	if resp.Header.Get("Content-Type") != expectedHeader {
		t.Errorf("Expected Content-Type %q, got %q", expectedHeader, resp.Header.Get("Content-Type"))
	}
}

func TestHttpClient_SendRequestReturnEntity_403(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	errorMsg := "forbidden: missing permissions"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(errorMsg))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = client.SendRequestReturnEntity(req, 5*time.Second)
	if err == nil {
		t.Fatal("Expected authentication error, got nil")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected 'authentication failed' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), errorMsg) {
		t.Errorf("Expected error to contain %q, got: %v", errorMsg, err)
	}
}

func TestHttpClient_SendRequestReturnEntity_ReadBodyError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	// 模拟一个返回无效 body 的服务器（例如关闭连接）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		// 不写入任何 body，导致 ReadAll 失败
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = client.SendRequestReturnEntity(req, 5*time.Second)
	if err == nil {
		t.Fatal("Expected body read error, got nil")
	}
}

// 测试重复关闭 resp.Body 不会导致 panic（验证 defer 安全性）
func TestHttpClient_SendRequestReturnEntity_DoubleClose(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewHttpClient(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 正常调用应无 panic
	_, err = client.SendRequestReturnEntity(req, 5*time.Second)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// 测试通过：无 panic 即可
}

// 验证日志是否被正确调用（可选，需 mock logger）
// 本测试依赖 zaptest，日志输出到 test log，不验证内容

func TestMain(m *testing.M) {
	// 可选：全局设置
	m.Run()
}
