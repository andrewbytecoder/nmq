package nmqmessage

import (
	"errors"
	"fmt"
	"github.com/nmq/interfaces"
	"github.com/nmq/utils"
	"go.uber.org/zap"
	"io"
	"net"
	"os"
	"sync"
)

type MessageServer struct {
	ctx       interfaces.NmqContext
	log       *zap.Logger
	component interfaces.Component

	listener net.Listener // 监听链接

	snowNode *utils.SnowNode

	mux         sync.RWMutex // 可以同时获取，但是不能同时写入
	connections map[utils.SnowID]net.Conn
}

func NewMessageServer(ctx interfaces.NmqContext, cp interfaces.Component) *MessageServer {
	return &MessageServer{
		ctx:       ctx,
		log:       ctx.GetLogger(),
		component: cp,
	}
}

func (ms *MessageServer) init(network, address string) error {
	// 创建连接池对象
	ms.mux.Lock()
	ms.connections = make(map[net.Conn]net.Conn)
	ms.mux.Unlock()
	// 获取雪花生成器
	snowNode, ok := ms.component.GetInterface("network_snow_flake").(*utils.SnowNode)
	if !ok {
		ms.log.Error("network snow flake not found")
		return errors.New("network snow flake not found")
	}
	ms.snowNode = snowNode

	var err error
	ms.listener, err = net.Listen(network, address)
	if err != nil {
		ms.log.Error("listen error", zap.Error(err))
		return err
	}

	return nil
}

func (ms *MessageServer) Start(network, address string) error {
	// 初始化网络配置
	err := ms.init(network, address)
	if err != nil {
		ms.log.Error("init error", zap.Error(err))
		return err
	}

	// 启动独立的 goroutine 处理 Accept
	go func() {
		for {
			conn, err := ms.listener.Accept()
			if err != nil {
				var opErr *net.OpError

				if errors.As(err, &opErr) && opErr.Timeout() {
					ms.log.Warn("accept timeout", zap.String("network", network))
					// 超时继续
					continue
				}
				// 错误退出
				ms.log.Error("accept error", zap.Error(err))
				return
			}
			ms.mux.Lock()
			snowId := ms.snowNode.Generate()
			ms.connections[snowId] = conn
			ms.mux.Unlock()
			go ms.handleConnection(conn)
		}
	}()

	for {
		select {
		case conn := <-connChan:
			// 处理新连接
			go handleConnection(conn)
		case err := <-errChan:
			ms.log.Error("accept error", zap.Error(err))
		case <-ms.ctx.GetContext().Done():
			// Context 被取消，退出循环
			ms.log.Info("context cancelled, stopping server")
			return ms.ctx.GetContext().Err()
		}
	}

	return nil
}

// 处理客户端连接
func (ms *MessageServer) handleConnection(snowId utils.SnowID, conn net.Conn) {
	defer conn.Close()

	// 接收二进制数据并写入文件
	buffer := make([]byte, 4096) // 4KB 缓冲区
	var totalReceived int64

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("客户端断开连接，共接收 %d 字节\n", totalReceived)
			} else {
				fmt.Println("读取数据出错:", err)
			}
			break
		}

		// 写入文件
		written, writeErr := file.Write(buffer[:n])
		if writeErr != nil {
			fmt.Println("写入文件失败:", writeErr)
			break
		}

		totalReceived += int64(written)
		fmt.Printf("已接收 %d 字节\n", totalReceived)
	}
}

func (ms *MessageServer) Stop() error {
	// 先关闭监听
	if ms.listener != nil {
		return ms.listener.Close()
	}

	return nil
}
