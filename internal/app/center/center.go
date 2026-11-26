package center

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"svnpanel/internal/service"
	"svnpanel/internal/svncfg"
	"svnpanel/pkg/resp"
	"svnpanel/web"

	"github.com/gin-gonic/gin"
)

const sessionCookie = "svnpanel_session"

func Config(r *gin.Engine, svc *service.Service) {
	api := r.Group("/api/v1")
	api.GET("/healthz", func(c *gin.Context) {
		resp.OK(c, gin.H{"status": "ok"})
	})
	api.POST("/auth/login", login(svc))
	api.GET("/auth/session", sessionInfo(svc))

	protected := api.Group("")
	protected.Use(requireAuth(svc))
	protected.GET("/auth/me", me)
	protected.POST("/auth/logout", requireCSRF(), logout(svc))

	protected.GET("/repositories", listRepositories(svc))
	protected.GET("/repositories/:repo/tree", listPaths(svc))
	protected.GET("/repositories/:repo/acl", getACL(svc))
	protected.POST("/repositories/:repo/acl", requireCSRF(), setACL(svc))
	protected.DELETE("/repositories/:repo/acl", requireCSRF(), deleteACL(svc))

	protected.GET("/users", listUsers(svc))
	protected.POST("/users", requireCSRF(), createUser(svc))
	protected.POST("/users/batch", requireCSRF(), batchUsers(svc))
	protected.PATCH("/users/:name", requireCSRF(), patchUser(svc))
	protected.PUT("/users/:name/metadata", requireCSRF(), setUserMetadata(svc))
	protected.PUT("/users/:name/password", requireCSRF(), resetPassword(svc))
	protected.POST("/users/:name/reveal", requireCSRF(), revealPassword(svc))
	protected.DELETE("/users/:name", requireCSRF(), deleteUser(svc))

	protected.POST("/departments", requireCSRF(), addDepartment(svc))
	protected.PUT("/departments/:name", requireCSRF(), renameDepartment(svc))
	protected.DELETE("/departments/:name", requireCSRF(), deleteDepartment(svc))

	protected.GET("/groups", listGroups(svc))
	protected.POST("/groups", requireCSRF(), createGroup(svc))
	protected.PUT("/groups/:name/members", requireCSRF(), setGroupMembers(svc))
	protected.PUT("/groups/:name/permissions", requireCSRF(), setGroupPermissions(svc))
	protected.DELETE("/groups/:name", requireCSRF(), deleteGroup(svc))

	protected.GET("/audits", listAudits(svc))

	r.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			resp.Fail(c, http.StatusNotFound, "not_found", "接口不存在", nil)
			return
		}
		web.SPAHandler()(c)
	})
}

func requireAuth(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(sessionCookie)
		session, err := svc.Authenticate(token)
		if err != nil {
			resp.Fail(c, http.StatusUnauthorized, "unauthorized", "未登录或登录已过期", nil)
			return
		}
		c.Set("session", session)
		c.Set("token", token)
		c.Next()
	}
}

func requireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := currentSession(c)
		if c.GetHeader("X-CSRF-Token") == "" || c.GetHeader("X-CSRF-Token") != session.CSRF {
			resp.Fail(c, http.StatusForbidden, "csrf_failed", "CSRF 校验失败", nil)
			return
		}
		c.Next()
	}
}

func currentSession(c *gin.Context) service.Session {
	value, _ := c.Get("session")
	session, _ := value.(service.Session)
	return session
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUnauthorized):
		resp.Fail(c, http.StatusUnauthorized, "unauthorized", err.Error(), nil)
	case errors.Is(err, service.ErrRateLimited):
		resp.Fail(c, http.StatusTooManyRequests, "rate_limited", err.Error(), nil)
	case errors.Is(err, svncfg.ErrNotFound):
		resp.Fail(c, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, svncfg.ErrConflict):
		resp.Fail(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, svncfg.ErrAmbiguous):
		resp.Fail(c, http.StatusUnprocessableEntity, "ambiguous_config", err.Error(), nil)
	case errors.Is(err, svncfg.ErrValidation):
		resp.Fail(c, http.StatusBadRequest, "validation_failed", err.Error(), nil)
	default:
		slog.Error("请求处理失败", "error", err)
		resp.Fail(c, http.StatusInternalServerError, "internal_error", "服务器内部错误", nil)
	}
}

type revisionsRequest struct {
	Passwd string `json:"passwd"`
	Authz  string `json:"authz"`
}

func (r revisionsRequest) value() svncfg.Revisions {
	return svncfg.Revisions{Passwd: r.Passwd, Authz: r.Authz}
}
