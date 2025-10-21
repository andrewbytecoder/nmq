package nmqmessage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/andrewbytecoder/nmq/interfaces"
	"github.com/andrewbytecoder/nmq/pkg/utils"
	"go.uber.org/zap"
)

type MessageServer struct {
	ctx       interfaces.NmqContext
	log       *zap.Logger
	component interfaces.Component

	listener net.Listener // 监听链接

	snowNode *utils.SnowNode

	mux         sync.RWMutex // 可以同时获取，但是不能同时写入
	connections map[utils.SnowID]net.Conn

	msCtx    context.Context    // 消息服务器上下文
	msCancel context.CancelFunc // 消息服务器取消函数
}

func NewMessageServer(ctx interfaces.NmqContext, cp interfaces.Component) *MessageServer {
	return &MessageServer{
		ctx:       ctx,
		log:       ctx.GetLogger(),
		component: cp,
	}
}

func (ms *MessageServer) init(network, address string) error {
	ctx, cancel := context.WithCancel(ms.ctx.GetContext())
	ms.msCtx = ctx
	ms.msCancel = cancel
	// 创建连接池对象
	ms.mux.Lock()
	ms.connections = make(map[utils.SnowID]net.Conn)
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
			go ms.handleConnection(snowId, conn)
		}
	}()

	return nil
}

// 处理客户端连接
func (ms *MessageServer) handleConnection(snowId utils.SnowID, conn net.Conn) {
	// 函数退出，关闭所有连接
	defer conn.Close()

	// 接收二进制数据并写入文件
	buffer := make([]byte, 4096) // 4KB 缓冲区
	var totalReceived int64

	for {
		// 设置很短的超时时间，比如1-10毫秒
		err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		if err != nil {
			ms.log.Error("set read deadline error", zap.Error(err))
			return
		}
		n, err := conn.Read(buffer)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				// 超时，没有数据，可以执行其他操作
				continue // 或者 break，取决于业务需求
			}
			if err == io.EOF {
				ms.log.Info("客户端已断开连接")
				fmt.Printf("客户端断开连接，共接收 %d 字节\n", totalReceived)
			} else {
				fmt.Println("读取数据出错:", err)
			}
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
	// 停止所有数据的接收任务
	ms.msCancel()

	return nil
}
