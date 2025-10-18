package httpclient

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HttpClient 是一个封装了HTTP客户端功能的结构体
type HttpClient struct {
	// logger 用于记录日志信息
	logger *zap.Logger
	// c 是实际执行HTTP请求的客户端实例
	c *http.Client
}

// NewHttpClient 创建一个新的HttpClient实例
// 参数:
//   - log: 用于记录日志的zap.Logger实例
//   - timeoutSec: HTTP请求的超时时间(秒)
//
// 返回值:
//   - *HttpClient: 新创建的HttpClient实例
func NewHttpClient(log *zap.Logger) *HttpClient {
	return &HttpClient{
		logger: log,
		c: &http.Client{
			Transport: &http.Transport{
				// 配置TLS客户端跳过证书验证
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// handleAuthenticationError 处理认证错误响应
// 参数:
//   - resp: HTTP响应对象
//
// 返回值:
//   - error: 包含认证错误详情的错误信息
func handleAuthenticationError(resp *http.Response) error {
	// 读取响应体内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	// 返回包含响应体内容的认证错误信息
	return fmt.Errorf("authentication failed | Response: %s", string(body))
}

// Send 发送HTTP请求并返回响应体
// 参数:
//   - request: 要发送的HTTP请求对象
//   - timeout: 请求超时时间
//
// 返回值:
//   - []byte: 响应体的字节数据
//   - error: 请求过程中可能发生的错误
func (hc *HttpClient) Send(request *http.Request, timeout time.Duration) ([]byte, error) {
	// 设置请求超时时间
	hc.c.Timeout = timeout
	// 执行HTTP请求
	resp, err := hc.c.Do(request)
	if err != nil {
		return nil, err
	}
	// 确保在函数结束时关闭响应体
	defer resp.Body.Close()

	// 检查是否为403 Forbidden状态码，如果是则处理认证错误
	if resp.StatusCode == http.StatusForbidden {
		authErr := handleAuthenticationError(resp)
		if authErr != nil {
			return nil, authErr
		}
	}

	// 读取并返回响应体内容
	return io.ReadAll(resp.Body)
}

// SendRequestReturnEntity 发送HTTP请求并返回包含详细信息的EntityResponse实体
// 参数:
//   - request: 要发送的HTTP请求对象
//   - timeout: 请求超时时间
//
// 返回值:
//   - *EntityResponse: 包含状态码、响应体和响应头的响应实体
//   - error: 请求过程中可能发生的错误
func (hc *HttpClient) SendRequestReturnEntity(request *http.Request, timeout time.Duration) (*EntityResponse, error) {
	// 设置超时时间
	hc.c.Timeout = timeout
	resp, err := hc.c.Do(request)
	if err != nil {
		return nil, err
	}
	// 确保在函数结束时关闭响应体
	defer resp.Body.Close()

	// 第一次检查403状态码并处理认证错误
	if resp.StatusCode == http.StatusForbidden {
		authErr := handleAuthenticationError(resp)
		if authErr != nil {
			return nil, authErr
		}
	}

	// 冗余的defer语句，实际上前面已经调用过
	defer resp.Body.Close()

	// 记录请求成功的日志，包含状态码
	hc.logger.Info("Request succeeded", zap.Int("status code", resp.StatusCode))

	// 创建新的EntityResponse实例
	entityResponse := NewEntityResponse()

	// 冗余的403状态码检查和认证错误处理逻辑
	if resp.StatusCode == http.StatusForbidden {
		authErr := handleAuthenticationError(resp)
		if authErr != nil {
			// 记录认证失败的详细错误日志，包含错误信息和响应头
			hc.logger.Error("Authentication failed", zap.Error(authErr), zap.Any("response Header", resp.Header))
			return nil, authErr
		}
	}

	// 设置响应状态码
	entityResponse.SetStatus(resp.StatusCode)

	// 读取响应体内容
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		// 记录读取响应体失败的错误日志
		hc.logger.Error("Read response body failed", zap.Error(err))
		return nil, err
	}

	// 设置响应体和响应头
	entityResponse.SetBody(out)
	entityResponse.SetHeader(resp.Header)

	// 返回完整的响应实体
	return entityResponse, nil
}

//func (hc *HttpClient) SendWithRetry(request *http.Request, timeout time.Duration, maxRetries int) ([]byte, error) {
//}
//
