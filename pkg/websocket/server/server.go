// Package server implements the websocket server functionality
// server包实现了websocket服务器功能
package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Server represents a websocket server that can accept client connections
// Server表示一个可以接受客户端连接的websocket服务器
type Server struct {
	// log is the logger instance for logging server activities
	// 用于记录服务器活动的日志实例
	log *zap.Logger
	// cfg holds the server configuration including address and port
	// 包含地址和端口的服务器配置
	cfg *Config
	// cliSet is a set of active websocket connections
	// 存储活跃websocket连接的集合
	cliSet map[*websocket.Conn]struct{}
}

// NewServer creates a new Server instance with the provided logger and configuration
// 使用提供的日志记录器和配置创建新的Server实例
// 参数log是zap日志记录器，cfg是服务器配置
func NewServer(log *zap.Logger, cfg *Config) *Server {
	return &Server{
		log: log,
		cfg: cfg,
	}
}

// upgrader is used to upgrade HTTP connections to websocket connections
// 用于将HTTP连接升级为websocket连接
var upgrader = websocket.Upgrader{} // use default options 使用默认选项

// Start begins the websocket server and starts listening for connections
// 启动websocket服务器并开始监听连接
// 绑定/ws路径处理函数并启动HTTP服务器
func (s *Server) Start() error {
	// Format the address with host and port
	// 格式化包含主机和端口的地址
	addr := fmt.Sprintf("%s:%d", s.cfg.Addr, s.cfg.Port)

	// Register the websocket handler function
	// 注册websocket处理函数
	http.HandleFunc("/ws", s.ws)
	// Start the HTTP server (this call blocks and logs fatal errors)
	// 启动HTTP服务器（此调用会阻塞并记录致命错误）
	log.Fatal(http.ListenAndServe(addr, nil))

	return nil
}

// ws is the HTTP handler function that upgrades connections to websocket and handles messages
// ws是将连接升级为websocket并处理消息的HTTP处理函数
// 参数w是HTTP响应写入器，r是HTTP请求
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a websocket connection
	// 将HTTP连接升级为websocket连接
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	s.cliSet[c] = struct{}{}
	s.cfg.onConnect(c)
}

// Stop shuts down the websocket server (currently unimplemented)
// 停止websocket服务器（目前未实现）
func (s *Server) Stop() error {
	for conn := range s.cliSet {
		s.cfg.onDisconnect(conn)
		s.Close(conn)
	}
	s.log.Info("server stopped")
	return nil
}

func (s *Server) Close(conn *websocket.Conn) error {
	delete(s.cliSet, conn)
	return conn.Close()
}

// ReadMessage reads a message from the websocket connection
// 从websocket连接中读取消息
// 实现了websocket.Server接口的ReadMessage方法
// 注意：当前实现存在缺陷，因为s.ws未被正确初始化
func (s *Server) ReadMessage(conn *websocket.Conn) (messageType int, p []byte, err error) {
	return conn.ReadMessage()
}

// WriteMessage writes a message to the websocket connection
// 向websocket连接写入消息
// 实现了websocket.Server接口的WriteMessage方法
// 参数messageType是消息类型，data是消息数据
// 注意：当前实现存在缺陷，因为s.ws未被正确初始化
func (s *Server) WriteMessage(conn *websocket.Conn, messageType int, data []byte) error {
	return conn.WriteMessage(messageType, data)
}
