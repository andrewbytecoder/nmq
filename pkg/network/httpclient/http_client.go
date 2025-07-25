package httpclient

import (
	"github.com/nmq/interfaces"
	"go.uber.org/zap"
	"io"
	"net/http"
)

type HttpClient struct {
	ctx    interfaces.NmqContext
	log    *zap.Logger
	client *http.Client
}

func NewHttpClient(ctx interfaces.NmqContext) *HttpClient {
	return &HttpClient{
		ctx:    ctx,
		log:    ctx.GetLogger(),
		client: &http.Client{},
	}
}

func (h *HttpClient) SendData(request *http.Request, snowId string) ([]byte, error) {
	h.log.Info("send request", zap.String("snowId", snowId))
	// 发送请求
	resp, err := h.client.Do(request)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
