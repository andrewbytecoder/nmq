package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDashboardStaticRoutes(t *testing.T) {
	originalMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		gin.SetMode(originalMode)
	})

	server, err := NewServer(Config{
		DashboardPath: "/dashboard",
		APIBasePath:   "/api",
	})
	require.NoError(t, err)

	t.Run("root redirects to dashboard", func(t *testing.T) {
		recorder := performRequest(t, server, "/")

		require.Equal(t, http.StatusFound, recorder.Code)
		require.Equal(t, "/dashboard/", recorder.Header().Get("Location"))
	})

	t.Run("dashboard redirects to trailing slash", func(t *testing.T) {
		recorder := performRequest(t, server, "/dashboard")

		require.Equal(t, http.StatusFound, recorder.Code)
		require.Equal(t, "/dashboard/", recorder.Header().Get("Location"))
	})

	t.Run("dashboard slash serves index", func(t *testing.T) {
		recorder := performRequest(t, server, "/dashboard/")

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
		require.Contains(t, recorder.Body.String(), "window.APIUrl")
		require.Contains(t, recorder.Body.String(), "/api")
	})

	t.Run("deep links fall back to index", func(t *testing.T) {
		recorder := performRequest(t, server, "/dashboard/http/routers")

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
		require.Contains(t, recorder.Body.String(), "window.APIUrl")
		require.Contains(t, recorder.Body.String(), "/api")
	})

	t.Run("existing assets are served directly", func(t *testing.T) {
		recorder := performRequest(t, server, "/dashboard/favicon.ico")

		require.Equal(t, http.StatusOK, recorder.Code)
		require.NotEmpty(t, recorder.Body.Bytes())
		require.False(t, strings.Contains(recorder.Body.String(), "<!DOCTYPE html>"))
	})
}

func performRequest(t *testing.T, server *Server, target string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, req)

	return recorder
}
