package nmq

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

// debugHandler 处理 /debug/loglevel 请求
type debugHandler struct {
	nmq *Nmq
}

func (h *debugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getLogLevel(w, r)
	case http.MethodPut:
		h.setLogLevel(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// getLogLevel 获取当前日志等级
func (h *debugHandler) getLogLevel(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]string{"level": h.nmq.GetLogLevel()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// setLogLevel 设置日志等级
func (h *debugHandler) setLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if err := h.nmq.SetLogLevel(req.Level); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := map[string]string{"level": h.nmq.GetLogLevel()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// startDebugServer 启动调试 HTTP 服务，端口由 cfg.debugPort 决定（0 表示不启动）
func startDebugServer(nmq *Nmq) {
	port := nmq.cfg.debugPort
	if port <= 0 {
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/debug/loglevel", &debugHandler{nmq: nmq})

	nmq.debugServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		nmq.logger.Info("starting debug http server",
			zap.Int("port", port),
			zap.String("endpoint", "/debug/loglevel"))
		if err := nmq.debugServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			nmq.logger.Error("debug http server error", zap.Error(err))
		}
	}()
}

// stopDebugServer 停止调试 HTTP 服务
func stopDebugServer(nmq *Nmq) {
	if nmq.debugServer != nil {
		nmq.logger.Info("stopping debug http server")
		// 使用 context.Background()，因为服务的 ctx 可能已被 cancel
		nmq.debugServer.Close()
		nmq.debugServer = nil
	}
}
