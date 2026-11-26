//go:build !embed

package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func SPAHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		if requestPath == "" || requestPath == "." {
			requestPath = "index.html"
		}
		fullPath := filepath.Join("frontend/dist", filepath.FromSlash(requestPath))
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			c.File(fullPath)
			return
		}
		if strings.HasPrefix(requestPath, "assets/") {
			c.Header("Cache-Control", "no-store")
			c.String(http.StatusNotFound, "static asset not found: %s", requestPath)
			return
		}
		indexPath := filepath.Join("frontend/dist", "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.Header("Cache-Control", "no-cache")
			c.File(indexPath)
			return
		}
		c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("前端尚未构建，请运行 make frontend-build"))
	}
}
