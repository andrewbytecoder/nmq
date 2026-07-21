package dashboard

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"ysp.com/ncp/ncp/internal/runtimecfg"
	"ysp.com/ncp/ncp/plugins/webui/storage"
	webui "ysp.com/ncp/ncp/plugins/webui/ui"
)

type Config struct {
	Logger          *zap.Logger
	DashboardPath   string
	APIBasePath     string
	ListenAddr      string
	StorageProvider storage.SQLiteTableProvider
}

type Server struct {
	engine          *gin.Engine
	logger          *zap.Logger
	dashboardPath   string
	apiBasePath     string
	listenAddr      string
	indexTemplate   *template.Template
	assets          fs.FS
	storageProvider storage.SQLiteTableProvider
}

type indexTemplateData struct {
	APIUrl string
}

type resourceStats struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Total    int `json:"total"`
}

type protocolOverview struct {
	Routers     resourceStats `json:"routers"`
	Services    resourceStats `json:"services"`
	Middlewares resourceStats `json:"middlewares,omitempty"`
}

func NewServer(cfg Config) (*Server, error) {
	assets := webui.FS
	indexTemplate, err := template.ParseFS(assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	server := &Server{
		engine:          gin.New(),
		logger:          logger,
		dashboardPath:   normalizePath(cfg.DashboardPath, "/dashboard"),
		apiBasePath:     normalizePath(cfg.APIBasePath, "/api"),
		listenAddr:      cfg.ListenAddr,
		indexTemplate:   indexTemplate,
		assets:          assets,
		storageProvider: cfg.StorageProvider,
	}

	server.engine.Use(gin.Recovery())

	if gin.Mode() == gin.DebugMode {
		server.engine.Use(gin.Logger())
	}

	server.registerStaticRoutes()
	server.registerAPIRoutes()

	return server, nil
}

func (s *Server) Run(addr string) error {
	listenAddr := addr
	if listenAddr == "" {
		listenAddr = s.listenAddr
	}
	if listenAddr == "" {
		listenAddr = "127.0.0.1:18080"
	}

	runtimecfg.SetWebuiAddress(runtimecfg.ServerAddress{
		Name:    "webui",
		Address: listenAddr,
	})

	return s.engine.Run(listenAddr)
}

func (s *Server) registerStaticRoutes() {
	trimmedDashboardPath := strings.TrimSuffix(s.dashboardPath, "/")

	s.engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, trimmedDashboardPath+"/")
	})

	s.engine.GET(trimmedDashboardPath, func(c *gin.Context) {
		c.Redirect(http.StatusFound, trimmedDashboardPath+"/")
	})

	s.engine.GET(trimmedDashboardPath+"/*filepath", s.serveDashboardAsset)
}

func (s *Server) registerAPIRoutes() {
	api := s.engine.Group(s.apiBasePath)

	api.GET("/rawdata", s.handleRawData)
	api.GET("/overview", s.handleOverview)
	api.GET("/support-dump", s.handleObjectPlaceholder)

	api.GET("/entrypoints", s.handleEntrypoints)
	api.GET("/entrypoints/:entryPointID", s.handleObjectPlaceholder)

	api.GET("/http/routers", s.handleCollectionPlaceholder)
	api.GET("/http/routers/:routerID", s.handleObjectPlaceholder)
	api.GET("/http/services", s.handleCollectionPlaceholder)
	api.GET("/http/services/:serviceID", s.handleObjectPlaceholder)
	api.GET("/http/middlewares", s.handleCollectionPlaceholder)
	api.GET("/http/middlewares/:middlewareID", s.handleObjectPlaceholder)

	api.GET("/tcp/routers", s.handleCollectionPlaceholder)
	api.GET("/tcp/routers/:routerID", s.handleObjectPlaceholder)
	api.GET("/tcp/services", s.handleCollectionPlaceholder)
	api.GET("/tcp/services/:serviceID", s.handleObjectPlaceholder)
	api.GET("/tcp/middlewares", s.handleCollectionPlaceholder)
	api.GET("/tcp/middlewares/:middlewareID", s.handleObjectPlaceholder)

	api.GET("/udp/routers", s.handleCollectionPlaceholder)
	api.GET("/udp/routers/:routerID", s.handleObjectPlaceholder)
	api.GET("/udp/services", s.handleCollectionPlaceholder)
	api.GET("/udp/services/:serviceID", s.handleObjectPlaceholder)

	api.GET("/certificates", s.handleCollectionPlaceholder)
	api.GET("/certificates/:certificateID", s.handleObjectPlaceholder)

	api.GET("/version", s.handleVersion)
	api.GET("/storage/sqlite", s.handleSQLiteTables)
	api.GET("/storage/sqlite/:tableName", s.handleSQLiteTableDetail)
}

func (s *Server) serveIndex(c *gin.Context) {
	s.setSecurityHeaders(c.Writer)
	c.Header("Content-Type", "text/html; charset=utf-8")

	apiPath := strings.TrimSuffix(s.apiBasePath, "/")
	if err := s.indexTemplate.Execute(c.Writer, indexTemplateData{APIUrl: apiPath}); err != nil {
		s.logger.Error("render dashboard index failed", zap.Error(err))
		c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
}

func (s *Server) serveDashboardAsset(c *gin.Context) {
	s.setSecurityHeaders(c.Writer)

	requestedPath := strings.TrimPrefix(c.Param("filepath"), "/")
	if requestedPath == "" {
		s.serveIndex(c)
		return
	}

	if !s.assetExists(requestedPath) || !strings.Contains(path.Base(requestedPath), ".") {
		s.serveIndex(c)
		return
	}

	c.Header("Content-Type", "")

	fileServer := http.FileServerFS(s.assets)
	http.StripPrefix(strings.TrimSuffix(s.dashboardPath, "/")+"/", fileServer).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "frame-src 'self';")
}

func (s *Server) assetExists(assetPath string) bool {
	file, err := s.assets.Open(assetPath)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func (s *Server) handleRawData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"dashboardPath": s.dashboardPath,
		"apiBasePath":   s.apiBasePath,
		"backend":       "gin",
		"storageMode":   "dpcore-interface",
	})
}

func (s *Server) handleOverview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"http": protocolOverview{
			Routers:     resourceStats{},
			Services:    resourceStats{},
			Middlewares: resourceStats{},
		},
		"tcp": protocolOverview{
			Routers:     resourceStats{},
			Services:    resourceStats{},
			Middlewares: resourceStats{},
		},
		"udp": gin.H{
			"routers":  resourceStats{},
			"services": resourceStats{},
		},
		"features": gin.H{
			"backend": "gin",
			"sqlite":  true,
		},
		"providers": []string{"dpcore"},
	})
}

type EntryPointResp struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (s *Server) handleEntrypoints(c *gin.Context) {
	entryPoints := make([]EntryPointResp, 0)

	entryPoints = append(entryPoints, EntryPointResp{
		Name:    runtimecfg.GetNcpHttpAddress().Name,
		Address: runtimecfg.GetNcpHttpAddress().Address,
	})

	entryPoints = append(entryPoints, EntryPointResp{
		Name:    runtimecfg.GetNcpHttpsAddress().Name,
		Address: runtimecfg.GetNcpHttpsAddress().Address,
	})

	entryPoints = append(entryPoints, EntryPointResp{
		Name:    runtimecfg.GetWebuiAddress().Name,
		Address: runtimecfg.GetWebuiAddress().Address,
	})

	c.JSON(http.StatusOK, entryPoints)
}

func (s *Server) handleCollectionPlaceholder(c *gin.Context) {
	c.Header("X-Next-Page", "1")
	c.JSON(http.StatusOK, []any{})
}

func (s *Server) handleObjectPlaceholder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func (s *Server) handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"backend": "gin",
		"plugin":  "webui",
	})
}

func (s *Server) handleSQLiteTables(c *gin.Context) {
	page := intQuery(c, "page", 1)
	perPage := intQuery(c, "per_page", 10)
	sortBy := strings.ToLower(strings.TrimSpace(c.Query("sortBy")))
	if sortBy == "" {
		sortBy = "name"
	}
	direction := strings.ToLower(strings.TrimSpace(c.Query("direction")))
	if direction != "desc" {
		direction = "asc"
	}
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	if s.storageProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "storage provider is unavailable",
		})
		return
	}

	items, err := s.storageProvider.ListSQLiteTables()
	if err != nil {
		s.logger.Error("list sqlite tables failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to inspect dpcore storage metadata: %v", err),
		})
		return
	}

	filtered := make([]storage.SQLiteTableSummary, 0, len(items))
	for _, item := range items {
		if search == "" || strings.Contains(strings.ToLower(item.Name), search) {
			filtered = append(filtered, item)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]

		compare := 0
		switch sortBy {
		case "rowcount", "rows":
			compare = compareInt64(left.RowCount, right.RowCount)
		case "columncount", "columns":
			compare = compareInt(left.ColumnCount, right.ColumnCount)
		case "primarycount", "primarykeys":
			compare = compareInt(left.PrimaryCount, right.PrimaryCount)
		case "type":
			compare = strings.Compare(left.Type, right.Type)
		default:
			compare = strings.Compare(left.Name, right.Name)
		}

		if compare == 0 {
			compare = strings.Compare(left.Name, right.Name)
		}

		if direction == "desc" {
			return compare > 0
		}

		return compare < 0
	})

	pageItems, nextPage := paginate(filtered, page, perPage)
	c.Header("X-Next-Page", strconv.Itoa(nextPage))
	c.JSON(http.StatusOK, pageItems)
}

func (s *Server) handleSQLiteTableDetail(c *gin.Context) {
	page := intQuery(c, "page", 1)
	perPage := intQuery(c, "per_page", 20)
	tableName := strings.TrimSpace(c.Param("tableName"))

	if s.storageProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "storage provider is unavailable",
		})
		return
	}

	item, err := s.storageProvider.GetSQLiteTable(tableName, page, perPage)
	if err != nil {
		if errors.Is(err, storage.ErrSQLiteTableNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("sqlite table %q was not found", tableName),
			})
			return
		}

		s.logger.Error("load sqlite table detail failed", zap.String("tableName", tableName), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to inspect sqlite table %q: %v", tableName, err),
		})
		return
	}

	c.JSON(http.StatusOK, item)
}

func paginate[T any](items []T, page int, perPage int) ([]T, int) {
	if perPage <= 0 {
		perPage = 10
	}
	if page <= 0 {
		page = 1
	}

	start := (page - 1) * perPage
	if start >= len(items) {
		return []T{}, 1
	}

	end := start + perPage
	if end > len(items) {
		end = len(items)
	}

	nextPage := 1
	if end < len(items) {
		nextPage = page + 1
	}

	return items[start:end], nextPage
}

func intQuery(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func normalizePath(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	return strings.TrimRight(trimmed, "/")
}

func compareInt(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64(left int64, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
