package producer

import (
	"github.com/andrewbytecoder/nmq/pkg/websocket/client"
	"go.uber.org/zap"
)

type Producer struct {
	log    *zap.Logger
	client *client.Client
}
