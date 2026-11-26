package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"svnpanel/internal/cfg"

	"github.com/gin-gonic/gin"
)

func NewEngine(c cfg.Config) *gin.Engine {
	if c.Global.RunMode != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = r.SetTrustedProxies(c.HTTP.TrustedProxies)
	r.Use(gin.Recovery())
	r.Use(securityHeaders)
	r.Use(requestLimit(c.HTTP.MaxContentLength))
	r.Use(accessLog())
	return r
}

func securityHeaders(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.Header("Cache-Control", "no-store")
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; font-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
	c.Next()
}

func requestLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"client_ip", c.ClientIP(),
		)
	}
}

type Server struct {
	httpServer *http.Server
	shutdown   time.Duration
}

func Start(c cfg.Config, handler http.Handler) (*Server, error) {
	addr := fmt.Sprintf("%s:%d", c.HTTP.Host, c.HTTP.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: time.Duration(c.HTTP.ReadTimeout) * time.Second,
		ReadTimeout:       time.Duration(c.HTTP.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(c.HTTP.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(c.HTTP.IdleTimeout) * time.Second,
	}
	result := &Server{httpServer: srv, shutdown: time.Duration(c.HTTP.ShutdownTimeout) * time.Second}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP 服务已启动", "address", "http://"+addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return nil, err
	case <-time.After(100 * time.Millisecond):
		return result, nil
	}
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()
	err := s.httpServer.Shutdown(ctx)
	if err != nil && !strings.Contains(err.Error(), "Server closed") {
		return err
	}
	return nil
}
