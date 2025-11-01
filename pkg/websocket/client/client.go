// Package client implements the websocket client functionality
// client包实现了websocket客户端功能
package client

import (
	"fmt"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client represents a websocket client that can connect to a websocket server
// Client表示一个可以连接到websocket服务器的客户端
type Client struct {
	// log is the logger instance for logging client activities
	// 用于记录客户端活动的日志实例
	log *zap.Logger
	// cfg holds the client configuration including address and port
	// 包含地址和端口的客户端配置
	cfg *Config
	// ws is the underlying websocket connection
	// 底层的websocket连接
	ws *websocket.Conn
}

// NewClient creates a new Client instance with the provided logger and configuration
// 使用提供的日志记录器和配置创建新的Client实例
// 参数log是zap日志记录器，cfg是客户端配置
func NewClient(log *zap.Logger, cfg *Config) *Client {
	return &Client{
		log: log,
		cfg: cfg,
	}
}

// Dial establishes a connection to the websocket server using the client's configuration
// 使用客户端配置建立与websocket服务器的连接
// 构造websocket URL并使用gorilla/websocket库进行连接
func (c *Client) Dial() error {
	// Format the address with host and port
	// 格式化包含主机和端口的地址
	addr := fmt.Sprintf("%s:%d", c.cfg.Addr, c.cfg.Port)

	// Construct the websocket URL
	// 构造websocket URL
	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	// Dial the websocket server
	// 拨号连接websocket服务器
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
		return err
	}
	c.ws = ws
	return nil
}

// Close closes the websocket connection
// 关闭websocket连接
func (c *Client) Close() error {
	return c.ws.Close()
}

// ReadMessage reads a message from the websocket connection
// 从websocket连接中读取消息
// 实现了websocket.Client接口的ReadMessage方法
func (c *Client) ReadMessage() (messageType int, p []byte, err error) {
	return c.ws.ReadMessage()
}

// WriteMessage writes a message to the websocket connection
// 向websocket连接写入消息
// 实现了websocket.Client接口的WriteMessage方法
// 参数messageType是消息类型，data是消息数据
func (c *Client) WriteMessage(messageType int, data []byte) error {
	return c.ws.WriteMessage(messageType, data)
}
