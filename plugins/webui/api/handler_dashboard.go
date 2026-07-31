package api

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

	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	"github.com/andrewbytecoder/nmq/plugins/webui/ctx"
	"github.com/andrewbytecoder/nmq/plugins/webui/routeinfoprovider"
	"github.com/andrewbytecoder/nmq/plugins/webui/storage"
	webui "github.com/andrewbytecoder/nmq/plugins/webui/ui"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Config struct {
	Ctx             *ctx.Context
	Logger          *zap.Logger
	DashboardPath   string
	APIBasePath     string
	ListenAddr      string
	StorageProvider storage.SQLiteTableProvider
	RouteProvider   routeinfoprovider.IRouteInfoProvider
}

type Server struct {
	ctx             *ctx.Context
	engine          *gin.Engine
	logger          *zap.Logger
	dashboardPath   string
	apiBasePath     string
	listenAddr      string
	indexTemplate   *template.Template
	assets          fs.FS
	storageProvider storage.SQLiteTableProvider
	routeProvider   routeinfoprovider.IRouteInfoProvider
}

type indexTemplateData struct {
	APIUrl        string
	DashboardBase string
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

type apiOverview struct {
	Handlers resourceStats `json:"handlers"`
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
		routeProvider:   cfg.RouteProvider,
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

	s.engine.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

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

	api.GET("/fileview/meta", s.handleFileViewMeta)
	api.GET("/fileview/list", s.handleFileViewList)
	api.POST("/fileview/delete", s.handleFileViewDelete)
	api.POST("/fileview/rename", s.handleFileViewRename)
	api.POST("/fileview/create-file", s.handleFileViewCreateFile)
	api.POST("/fileview/create-folder", s.handleFileViewCreateFolder)
	api.POST("/fileview/upload", s.handleFileViewUpload)
	api.GET("/fileview/preview", s.handleFileViewPreview)
	api.GET("/fileview/download", s.handleFileViewDownload)
	api.GET("/fileview/search", s.handleFileViewSearch)
	api.POST("/fileview/save", s.handleFileViewSave)

	api.GET("/handlers/routers", s.handleHTTPHandlers)

	api.GET("/http/routers", func(c *gin.Context) { s.handleMockCollection(c, mockHTTPRouters) })
	api.GET("/http/routers/:routerID", func(c *gin.Context) { s.handleMockObject(c, "routerID", mockHTTPRouters) })
	api.GET("/http/services", func(c *gin.Context) { s.handleMockCollection(c, mockHTTPServices) })
	api.GET("/http/services/:serviceID", func(c *gin.Context) { s.handleMockObject(c, "serviceID", mockHTTPServices) })
	api.GET("/http/middlewares", func(c *gin.Context) { s.handleMockCollection(c, mockHTTPMiddlewares) })
	api.GET("/http/middlewares/:middlewareID", func(c *gin.Context) { s.handleMockObject(c, "middlewareID", mockHTTPMiddlewares) })

	api.GET("/tcp/routers", func(c *gin.Context) { s.handleMockCollection(c, mockTCPRouters) })
	api.GET("/tcp/routers/:routerID", func(c *gin.Context) { s.handleMockObject(c, "routerID", mockTCPRouters) })
	api.GET("/tcp/services", func(c *gin.Context) { s.handleMockCollection(c, mockTCPServices) })
	api.GET("/tcp/services/:serviceID", func(c *gin.Context) { s.handleMockObject(c, "serviceID", mockTCPServices) })
	api.GET("/tcp/middlewares", func(c *gin.Context) { s.handleMockCollection(c, mockTCPMiddlewares) })
	api.GET("/tcp/middlewares/:middlewareID", func(c *gin.Context) { s.handleMockObject(c, "middlewareID", mockTCPMiddlewares) })

	api.GET("/udp/routers", func(c *gin.Context) { s.handleMockCollection(c, mockUDPRouters) })
	api.GET("/udp/routers/:routerID", func(c *gin.Context) { s.handleMockObject(c, "routerID", mockUDPRouters) })
	api.GET("/udp/services", func(c *gin.Context) { s.handleMockCollection(c, mockUDPServices) })
	api.GET("/udp/services/:serviceID", func(c *gin.Context) { s.handleMockObject(c, "serviceID", mockUDPServices) })

	api.GET("/certificates", func(c *gin.Context) { s.handleMockCollection(c, mockCertificates) })
	api.GET("/certificates/:certificateID", func(c *gin.Context) { s.handleMockObject(c, "certificateID", mockCertificates) })

	api.GET("/version", s.handleVersion)
	api.GET("/storage/sqlite", s.handleSQLiteTables)
	api.GET("/storage/sqlite/:tableName", s.handleSQLiteTableDetail)
}

func (s *Server) serveIndex(c *gin.Context) {
	s.setSecurityHeaders(c.Writer)
	c.Header("Content-Type", "text/html; charset=utf-8")

	apiPath := strings.TrimSuffix(s.apiBasePath, "/")
	dashboardBase := strings.TrimSuffix(s.dashboardPath, "/") + "/"
	if err := s.indexTemplate.Execute(c.Writer, indexTemplateData{
		APIUrl:        apiPath,
		DashboardBase: dashboardBase,
	}); err != nil {
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
	handlerStats := resourceStats{
		Total:    len(mockRouterHandlers),
		Warnings: 0,
		Errors:   0,
	}

	if s.routeProvider != nil {
		if routes, err := s.routeProvider.ListRouters(); err == nil && len(routes) > 0 {
			handlerStats.Total = len(routes)
		} else if err != nil {
			s.logger.Warn("route provider unavailable, using mock handler stats", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"http": protocolOverview{
			Routers: resourceStats{
				Total:    len(mockHTTPRouters),
				Warnings: 2,
				Errors:   0,
			},
			Services: resourceStats{
				Total:    len(mockHTTPServices),
				Warnings: 1,
				Errors:   0,
			},
			Middlewares: resourceStats{
				Total:    len(mockHTTPMiddlewares),
				Warnings: 0,
				Errors:   0,
			},
		},
		"tcp": protocolOverview{
			Routers: resourceStats{
				Total:    len(mockTCPRouters),
				Warnings: 1,
				Errors:   0,
			},
			Services: resourceStats{
				Total:    len(mockTCPServices),
				Warnings: 0,
				Errors:   0,
			},
			Middlewares: resourceStats{
				Total:    len(mockTCPMiddlewares),
				Warnings: 0,
				Errors:   0,
			},
		},
		"udp": protocolOverview{
			Routers: resourceStats{
				Total:    len(mockUDPRouters),
				Warnings: 0,
				Errors:   0,
			},
			Services: resourceStats{
				Total:    len(mockUDPServices),
				Warnings: 0,
				Errors:   0,
			},
		},
		"api": apiOverview{
			Handlers: handlerStats,
		},
		"features": gin.H{
			"backend":       "gin",
			"sqlite":        true,
			"mockOverview":  true,
			"accesslog":     true,
			"apiDashboard":  true,
			"dynamicConfig": "ncp-style",
		},
		"providers": []string{"internal", "file", "docker", "kubernetes"},
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

func (s *Server) handleHTTPHandlers(c *gin.Context) {
	s.handleRoutesCollection(c, func() (routeinfoprovider.RoutesInfo, error) {
		if s.routeProvider == nil {
			return mockRouterHandlers, nil
		}

		routes, err := s.routeProvider.ListRouters()
		if err != nil {
			s.logger.Warn("route provider unavailable, using mock handler routes", zap.Error(err))
			return mockRouterHandlers, nil
		}
		if len(routes) == 0 {
			s.logger.Warn("route provider returned no routes, using mock handler routes")
			return mockRouterHandlers, nil
		}

		return routes, nil
	})
}

func (s *Server) handleRoutesCollection(c *gin.Context, list func() (routeinfoprovider.RoutesInfo, error)) {
	page := intQuery(c, "page", 1)
	perPage := intQuery(c, "per_page", 10)
	sortBy := strings.ToLower(strings.TrimSpace(c.Query("sortBy")))
	if sortBy == "" {
		sortBy = "path"
	}
	direction := strings.ToLower(strings.TrimSpace(c.Query("direction")))
	if direction != "desc" {
		direction = "asc"
	}
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	method := strings.ToUpper(strings.TrimSpace(c.Query("method")))

	items, err := list()
	if err != nil {
		s.logger.Error("list route handlers failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to load route handlers: %v", err),
		})
		return
	}

	filtered := make(routeinfoprovider.RoutesInfo, 0, len(items))
	for _, item := range items {
		if method != "" && strings.ToUpper(item.Method) != method {
			continue
		}

		if search != "" {
			searchHit := strings.Contains(strings.ToLower(item.Method), search) ||
				strings.Contains(strings.ToLower(item.Path), search) ||
				strings.Contains(strings.ToLower(item.Handler), search)
			if !searchHit {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]

		compare := 0
		switch sortBy {
		case "method":
			compare = strings.Compare(strings.ToUpper(left.Method), strings.ToUpper(right.Method))
		case "handler":
			compare = strings.Compare(strings.ToLower(left.Handler), strings.ToLower(right.Handler))
		default:
			compare = strings.Compare(strings.ToLower(left.Path), strings.ToLower(right.Path))
		}

		if compare == 0 {
			compare = strings.Compare(strings.ToLower(left.Path), strings.ToLower(right.Path))
		}
		if compare == 0 {
			compare = strings.Compare(strings.ToUpper(left.Method), strings.ToUpper(right.Method))
		}
		if compare == 0 {
			compare = strings.Compare(strings.ToLower(left.Handler), strings.ToLower(right.Handler))
		}

		if direction == "desc" {
			return compare > 0
		}

		return compare < 0
	})

	pageItems, nextPage := paginate(filtered, page, perPage)
	c.Header("X-Next-Page", strconv.Itoa(nextPage))
	c.Header("X-Total-Count", strconv.Itoa(len(filtered)))
	c.Header("X-Total-Pages", strconv.Itoa(totalPages(len(filtered), perPage)))
	c.JSON(http.StatusOK, pageItems)
}

func (s *Server) handleCollectionPlaceholder(c *gin.Context) {
	c.Header("X-Next-Page", "1")
	c.JSON(http.StatusOK, []any{})
}

func (s *Server) handleObjectPlaceholder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func (s *Server) handleMockCollection(c *gin.Context, items []gin.H) {
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
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))

	filtered := make([]gin.H, 0, len(items))
	for _, item := range items {
		if status != "" && strings.ToLower(mockString(item, "status")) != status {
			continue
		}

		if search != "" && !strings.Contains(strings.ToLower(fmt.Sprint(item)), search) {
			continue
		}

		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		compare := compareMockItems(filtered[i], filtered[j], sortBy)
		if direction == "desc" {
			return compare > 0
		}

		return compare < 0
	})

	pageItems, nextPage := paginate(filtered, page, perPage)
	c.Header("X-Next-Page", strconv.Itoa(nextPage))
	c.Header("X-Total-Count", strconv.Itoa(len(filtered)))
	c.Header("X-Total-Pages", strconv.Itoa(totalPages(len(filtered), perPage)))
	c.JSON(http.StatusOK, pageItems)
}

func (s *Server) handleMockObject(c *gin.Context, paramName string, items []gin.H) {
	targetName := strings.TrimSpace(c.Param(paramName))
	for _, item := range items {
		if mockString(item, "name") == targetName {
			c.JSON(http.StatusOK, item)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": fmt.Sprintf("resource %q was not found", targetName),
	})
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
	c.Header("X-Total-Count", strconv.Itoa(len(filtered)))
	c.Header("X-Total-Pages", strconv.Itoa(totalPages(len(filtered), perPage)))
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

func totalPages(totalItems int, perPage int) int {
	if perPage <= 0 {
		perPage = 10
	}
	if totalItems <= 0 {
		return 1
	}

	pages := totalItems / perPage
	if totalItems%perPage != 0 {
		pages++
	}
	if pages <= 0 {
		return 1
	}

	return pages
}

func compareMockItems(left gin.H, right gin.H, sortBy string) int {
	switch sortBy {
	case "priority":
		return compareInt(mockInt(left, "priority"), mockInt(right, "priority"))
	case "servers":
		return compareInt(mockServerCount(left), mockServerCount(right))
	case "sans":
		return compareInt(len(mockStringSlice(left, "sans")), len(mockStringSlice(right, "sans")))
	default:
		leftValue := strings.ToLower(mockSortValue(left, sortBy))
		rightValue := strings.ToLower(mockSortValue(right, sortBy))
		if compare := strings.Compare(leftValue, rightValue); compare != 0 {
			return compare
		}

		return strings.Compare(strings.ToLower(mockString(left, "name")), strings.ToLower(mockString(right, "name")))
	}
}

func mockSortValue(item gin.H, key string) string {
	switch key {
	case "servers":
		return strconv.Itoa(mockServerCount(item))
	case "priority":
		return strconv.Itoa(mockInt(item, "priority"))
	case "commonname":
		return mockString(item, "commonName")
	case "notafter":
		return mockString(item, "notAfter")
	case "issuercn":
		return mockString(item, "issuerCN")
	default:
		return mockString(item, key)
	}
}

func mockString(item gin.H, key string) string {
	value, ok := item[key]
	if !ok || value == nil {
		return ""
	}

	return fmt.Sprint(value)
}

func mockInt(item gin.H, key string) int {
	value, ok := item[key]
	if !ok || value == nil {
		return 0
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func mockStringSlice(item gin.H, key string) []string {
	value, ok := item[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, entry := range typed {
			result = append(result, fmt.Sprint(entry))
		}
		return result
	default:
		return nil
	}
}

func mockServerCount(item gin.H) int {
	loadBalancer, ok := item["loadBalancer"].(gin.H)
	if !ok {
		if raw, exists := item["loadBalancer"].(map[string]any); exists {
			loadBalancer = gin.H(raw)
		} else {
			return 0
		}
	}

	servers, ok := loadBalancer["servers"]
	if !ok || servers == nil {
		return 0
	}

	switch typed := servers.(type) {
	case []gin.H:
		return len(typed)
	case []map[string]any:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return 0
	}
}

var mockHTTPRouters = []gin.H{
	{
		"name":     "dashboard-secure@internal",
		"service":  "api@internal",
		"rule":     "Host(`ncp.local`) && PathPrefix(`/dashboard`)",
		"status":   "enabled",
		"provider": "internal",
		"using":    []string{"websecure"},
		"middlewares": []string{
			"security-headers@file",
			"compress@file",
		},
		"tls": gin.H{
			"options":      "modern@file",
			"certResolver": "le",
			"domains": []gin.H{
				{
					"main": "ncp.local",
					"sans": []string{"dashboard.ncp.local"},
				},
			},
		},
		"priority":    120,
		"priorityStr": "120",
	},
	{
		"name":        "whoami-public@docker",
		"service":     "whoami-svc@docker",
		"rule":        "Host(`whoami.local`)",
		"status":      "enabled",
		"provider":    "docker",
		"using":       []string{"web"},
		"middlewares": []string{"compress@file"},
		"priority":    100,
		"priorityStr": "100",
	},
	{
		"name":     "legacy-admin@file",
		"service":  "legacy-admin@file",
		"rule":     "Host(`legacy.local`) && PathPrefix(`/admin`)",
		"status":   "warning",
		"provider": "file",
		"using":    []string{"web"},
		"priority": 90,
		"error":    []string{"linked middleware auth-chain@file contains deprecated usersFile syntax"},
		"observability": gin.H{
			"accessLogs": true,
			"metrics":    true,
		},
	},
}

var mockHTTPServices = []gin.H{
	{
		"name":     "api@internal",
		"type":     "loadbalancer",
		"status":   "enabled",
		"provider": "internal",
		"loadBalancer": gin.H{
			"passHostHeader": true,
			"servers": []gin.H{
				{"url": "http://127.0.0.1:11092", "weight": 1},
			},
			"healthCheck": gin.H{
				"path":     "/dashboard/",
				"interval": "10s",
				"timeout":  "3s",
			},
		},
		"usedBy": []string{"dashboard-secure@internal"},
		"serverStatus": gin.H{
			"http://127.0.0.1:11092": "UP",
		},
	},
	{
		"name":     "whoami-svc@docker",
		"type":     "loadbalancer",
		"status":   "enabled",
		"provider": "docker",
		"loadBalancer": gin.H{
			"passHostHeader": true,
			"servers": []gin.H{
				{"url": "http://10.42.0.21:8080", "weight": 1},
				{"url": "http://10.42.0.22:8080", "weight": 1},
			},
		},
		"usedBy": []string{"whoami-public@docker"},
		"serverStatus": gin.H{
			"http://10.42.0.21:8080": "UP",
			"http://10.42.0.22:8080": "UP",
		},
	},
	{
		"name":     "legacy-admin@file",
		"type":     "weighted",
		"status":   "warning",
		"provider": "file",
		"weighted": gin.H{
			"services": []gin.H{
				{"name": "legacy-admin-v1@file", "weight": 8},
				{"name": "legacy-admin-v2@file", "weight": 2},
			},
		},
		"mirroring": gin.H{
			"mirrors": []gin.H{
				{"name": "legacy-admin-shadow@file", "percent": 5},
			},
		},
		"usedBy": []string{"legacy-admin@file"},
	},
}

var mockHTTPMiddlewares = []gin.H{
	{
		"name":     "security-headers@file",
		"type":     "headers",
		"status":   "enabled",
		"provider": "file",
		"usedBy":   []string{"dashboard-secure@internal"},
		"headers": gin.H{
			"frameDeny":               true,
			"browserXssFilter":        true,
			"contentTypeNosniff":      true,
			"stsSeconds":              31536000,
			"customFrameOptionsValue": "SAMEORIGIN",
		},
	},
	{
		"name":     "compress@file",
		"type":     "compress",
		"status":   "enabled",
		"provider": "file",
		"usedBy":   []string{"dashboard-secure@internal", "whoami-public@docker"},
		"compress": gin.H{
			"excludedContentTypes": []string{"text/event-stream"},
		},
	},
	{
		"name":     "auth-chain@file",
		"type":     "chain",
		"status":   "warning",
		"provider": "file",
		"usedBy":   []string{"legacy-admin@file"},
		"chain": gin.H{
			"middlewares": []string{"ip-allowlist@file", "basic-auth@file"},
		},
		"error": []string{"basic-auth@file references a legacy flat-file credential format"},
	},
}

var mockTCPRouters = []gin.H{
	{
		"name":        "mysql-ingress@file",
		"service":     "mysql-backend@file",
		"rule":        "HostSNI(`mysql.ncp.local`)",
		"status":      "enabled",
		"provider":    "file",
		"using":       []string{"mysql-tcp"},
		"middlewares": []string{"mysql-ip-allowlist@file"},
		"tls": gin.H{
			"options": "strict@file",
		},
		"priority":    70,
		"priorityStr": "70",
	},
	{
		"name":        "redis-secure@docker",
		"service":     "redis-backend@docker",
		"rule":        "HostSNI(`redis.ncp.local`)",
		"status":      "enabled",
		"provider":    "docker",
		"using":       []string{"redis-tcp"},
		"middlewares": []string{"redis-inflight-limit@file"},
		"priority":    60,
		"priorityStr": "60",
	},
	{
		"name":        "legacy-sip-tcp@file",
		"service":     "legacy-sip@file",
		"rule":        "HostSNI(`*`)",
		"status":      "warning",
		"provider":    "file",
		"using":       []string{"sip-tcp"},
		"priority":    40,
		"priorityStr": "40",
		"error":       []string{"TLS passthrough is disabled while upstream expects encrypted traffic"},
	},
}

var mockTCPServices = []gin.H{
	{
		"name":     "mysql-backend@file",
		"type":     "loadbalancer",
		"status":   "enabled",
		"provider": "file",
		"loadBalancer": gin.H{
			"terminationDelay": 100,
			"servers": []gin.H{
				{"address": "10.0.10.11:3306", "weight": 1},
				{"address": "10.0.10.12:3306", "weight": 1},
			},
			"healthCheck": gin.H{
				"interval": "15s",
				"timeout":  "5s",
			},
		},
		"usedBy": []string{"mysql-ingress@file"},
		"serverStatus": gin.H{
			"10.0.10.11:3306": "UP",
			"10.0.10.12:3306": "UP",
		},
	},
	{
		"name":     "redis-backend@docker",
		"type":     "loadbalancer",
		"status":   "enabled",
		"provider": "docker",
		"loadBalancer": gin.H{
			"terminationDelay": 50,
			"servers": []gin.H{
				{"address": "10.42.0.31:6379", "weight": 1},
			},
		},
		"usedBy": []string{"redis-secure@docker"},
		"serverStatus": gin.H{
			"10.42.0.31:6379": "UP",
		},
	},
	{
		"name":     "legacy-sip@file",
		"type":     "weighted",
		"status":   "warning",
		"provider": "file",
		"weighted": gin.H{
			"services": []gin.H{
				{"name": "legacy-sip-a@file", "weight": 1},
				{"name": "legacy-sip-b@file", "weight": 1},
			},
		},
		"usedBy": []string{"legacy-sip-tcp@file"},
	},
}

var mockTCPMiddlewares = []gin.H{
	{
		"name":     "mysql-ip-allowlist@file",
		"type":     "ipAllowList",
		"status":   "enabled",
		"provider": "file",
		"usedBy":   []string{"mysql-ingress@file"},
		"ipAllowList": gin.H{
			"sourceRange": []string{"10.0.0.0/8", "192.168.0.0/16"},
		},
	},
	{
		"name":     "redis-inflight-limit@file",
		"type":     "inFlightConn",
		"status":   "enabled",
		"provider": "file",
		"usedBy":   []string{"redis-secure@docker"},
		"inFlightConn": gin.H{
			"amount": 200,
		},
	},
}

var mockUDPRouters = []gin.H{
	{
		"name":        "syslog-udp@file",
		"service":     "syslog-receiver@file",
		"status":      "enabled",
		"provider":    "file",
		"using":       []string{"syslog-udp"},
		"priority":    20,
		"priorityStr": "20",
	},
	{
		"name":        "dns-forwarder@internal",
		"service":     "dns-upstream@internal",
		"status":      "enabled",
		"provider":    "internal",
		"using":       []string{"dns-udp"},
		"priority":    10,
		"priorityStr": "10",
	},
}

var mockUDPServices = []gin.H{
	{
		"name":     "syslog-receiver@file",
		"type":     "loadbalancer",
		"status":   "enabled",
		"provider": "file",
		"loadBalancer": gin.H{
			"terminationDelay": 0,
			"servers": []gin.H{
				{"address": "10.0.20.11:514", "weight": 1},
				{"address": "10.0.20.12:514", "weight": 1},
			},
		},
		"usedBy": []string{"syslog-udp@file"},
	},
	{
		"name":     "dns-upstream@internal",
		"type":     "weighted",
		"status":   "enabled",
		"provider": "internal",
		"weighted": gin.H{
			"services": []gin.H{
				{"name": "dns-primary@internal", "weight": 9},
				{"name": "dns-secondary@internal", "weight": 1},
			},
		},
		"usedBy": []string{"dns-forwarder@internal"},
	},
}

var mockCertificates = []gin.H{
	{
		"name":                 "ncp-local-cert",
		"commonName":           "ncp.local",
		"organization":         "NCP Labs",
		"country":              "CN",
		"status":               "enabled",
		"resolver":             "le",
		"notBefore":            "2026-06-01T00:00:00Z",
		"notAfter":             "2026-12-01T00:00:00Z",
		"sans":                 []string{"dashboard.ncp.local", "api.ncp.local"},
		"issuerCN":             "Let's Encrypt R12",
		"issuerOrg":            "Let's Encrypt",
		"issuerCountry":        "US",
		"version":              "3",
		"serialNumber":         "04:19:88:AB:22:F0",
		"keyType":              "RSA",
		"keySize":              2048,
		"signatureAlgorithm":   "SHA256-RSA",
		"certFingerprint":      "4A:8E:91:7B:21:54:AC:62",
		"publicKeyFingerprint": "A3:0F:65:11:71:C2:8D:40",
	},
	{
		"name":                 "wildcard-services-cert",
		"commonName":           "*.svc.ncp.local",
		"organization":         "NCP Platform",
		"country":              "CN",
		"status":               "enabled",
		"resolver":             "vault-pki",
		"notBefore":            "2026-07-10T00:00:00Z",
		"notAfter":             "2027-07-10T00:00:00Z",
		"sans":                 []string{"svc.ncp.local", "*.ops.ncp.local"},
		"issuerCN":             "NCP Internal PKI",
		"issuerOrg":            "NCP Platform",
		"issuerCountry":        "CN",
		"version":              "3",
		"serialNumber":         "09:77:AF:0C:42:10",
		"keyType":              "ECDSA",
		"keySize":              256,
		"signatureAlgorithm":   "ECDSA-SHA256",
		"certFingerprint":      "22:67:EF:0C:8A:10:BB:90",
		"publicKeyFingerprint": "7A:D0:11:CF:92:31:55:AB",
	},
	{
		"name":                 "legacy-admin-cert",
		"commonName":           "legacy.local",
		"organization":         "Legacy Admin Team",
		"country":              "CN",
		"status":               "expired",
		"resolver":             "file",
		"notBefore":            "2025-01-01T00:00:00Z",
		"notAfter":             "2026-05-01T00:00:00Z",
		"sans":                 []string{"admin.legacy.local"},
		"issuerCN":             "Legacy Internal CA",
		"issuerOrg":            "Legacy Operations",
		"issuerCountry":        "CN",
		"version":              "3",
		"serialNumber":         "00:00:DE:AD:BE:EF",
		"keyType":              "RSA",
		"keySize":              2048,
		"signatureAlgorithm":   "SHA256-RSA",
		"certFingerprint":      "DE:AD:BE:EF:11:22:33:44",
		"publicKeyFingerprint": "10:20:30:40:50:60:70:80",
	},
}

var mockRouterHandlers = routeinfoprovider.RoutesInfo{
	{Method: "GET", Path: "/healthz", Handler: "github.com/gin-gonic/gin.(*Engine).handleHTTPRequest"},
	{Method: "GET", Path: "/readyz", Handler: "ysp.com/ncp/ncp/plugins/dpproxy/httpapi.Ready"},
	{Method: "GET", Path: "/dashboard", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).serveIndex"},
	{Method: "GET", Path: "/dashboard/*filepath", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).serveDashboardAsset"},
	{Method: "GET", Path: "/api/overview", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleOverview"},
	{Method: "GET", Path: "/api/entrypoints", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleEntrypoints"},
	{Method: "GET", Path: "/api/handlers/routers", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleHTTPHandlers"},
	{Method: "GET", Path: "/api/http/routers", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/http/services", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/http/middlewares", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/tcp/routers", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/tcp/services", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/tcp/middlewares", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/udp/routers", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/udp/services", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
	{Method: "GET", Path: "/api/certificates", Handler: "ysp.com/ncp/ncp/plugins/webui/api.(*Server).handleMockCollection"},
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
