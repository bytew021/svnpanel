package center

import (
	"net/http"
	"strings"

	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func login(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request loginRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, session, err := svc.Login(strings.TrimSpace(request.Username), request.Password, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name: sessionCookie, Value: session.Token, Path: "/",
			HttpOnly: true, Secure: svc.Config.Session.CookieSecure, SameSite: http.SameSiteStrictMode,
			MaxAge: svc.Config.Session.AbsoluteHours * 3600,
		})
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

func me(c *gin.Context) {
	session := currentSession(c)
	c.Header("Cache-Control", "no-store")
	resp.OK(c, gin.H{
		"username": session.Username,
		"csrf":     session.CSRF,
	})
}

func sessionInfo(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(sessionCookie)
		session, err := svc.Authenticate(token)
		if err != nil {
			c.Header("Cache-Control", "no-store")
			resp.OK(c, gin.H{"authenticated": false})
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, gin.H{
			"authenticated": true,
			"username":      session.Username,
			"csrf":          session.CSRF,
		})
	}
}

func logout(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := currentSession(c)
		svc.Logout(session, c.ClientIP())
		http.SetCookie(c.Writer, &http.Cookie{
			Name: sessionCookie, Value: "", Path: "/", HttpOnly: true,
			Secure: svc.Config.Session.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1,
		})
		resp.OK(c, gin.H{"message": "已退出登录"})
	}
}
