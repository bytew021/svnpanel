//go:build embed

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var distFS embed.FS

func SPAHandler() gin.HandlerFunc {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return func(c *gin.Context) {
		requestPath := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		if requestPath == "" || requestPath == "." {
			requestPath = "index.html"
		}
		if file, err := sub.Open(requestPath); err == nil {
			_ = file.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		if strings.HasPrefix(requestPath, "assets/") {
			c.Header("Cache-Control", "no-store")
			c.String(http.StatusNotFound, "static asset not found: %s", requestPath)
			return
		}
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html 不存在")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
