package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersAllowEmbeddedIconFonts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(securityHeaders)
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "font-src 'self' data:") {
		t.Fatalf("CSP does not allow embedded fonts: %s", csp)
	}
}
