package httpclient

import "net/http"

// EntityResponse 表示一个HTTP响应的实体，包含了状态码、响应头和响应体。
type EntityResponse struct {
	// Status 表示HTTP响应的状态码。
	Status int `json:"status"`
	// Header 表示HTTP响应的头信息。
	Header http.Header `json:"header"`
	// Body 表示HTTP响应的主体内容，以字节数组的形式存储。
	Body []byte `json:"body"`
}

// NewEntityResponse 创建一个新的 EntityResponse 实例
func NewEntityResponse() *EntityResponse {
	return &EntityResponse{}
}

// SetStatus 设置 EntityResponse 的状态码
func (er *EntityResponse) SetStatus(status int) *EntityResponse {
	er.Status = status
	return er
}

// SetHeader 设置 EntityResponse 的响应头
func (er *EntityResponse) SetHeader(header http.Header) *EntityResponse {
	er.Header = header
	return er
}

// SetBody 设置 EntityResponse 的响应体
func (er *EntityResponse) SetBody(body []byte) *EntityResponse {
	er.Body = body
	return er
}
