package nmqmessage

import (
	"github.com/nmq/interfaces"
	"go.uber.org/zap"
	"net"
)

type MessageServer struct {
	ctx interfaces.NmqContext
	log *zap.Logger
}

func NewMessageServer(ctx interfaces.NmqContext) *MessageServer {
	return &MessageServer{
		ctx: ctx,
		log: ctx.GetLogger(),
	}
}

func (ms *MessageServer) NmqMessageServer(network, address string) error {
	listener, err := net.Listen(network, address)
	if err != nil {
		ms.log.Error("listen error", zap.Error(err))
		return err
	}
	for {
		_, err := listener.Accept()
		if err != nil {
			ms.log.Error("accept error", zap.Error(err))
			continue
		}
		go func() {
		}()
	}

	return nil
}
