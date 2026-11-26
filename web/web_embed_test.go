//go:build embed

package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEmbeddedAssetsHaveCorrectMIMEType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	matches, err := fs.Glob(distFS, "dist/assets/*.css")
	if err != nil || len(matches) == 0 {
		t.Fatalf("embedded CSS asset missing: %v", err)
	}
	assetURL := "/" + strings.TrimPrefix(matches[0], "dist/")

	r := gin.New()
	r.NoRoute(SPAHandler())

	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, assetURL, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("asset status = %d, body=%s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/css") {
		t.Fatalf("asset content type = %q", contentType)
	}

	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d, body=%s", missing.Code, missing.Body.String())
	}

	spa := httptest.NewRecorder()
	r.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/users", nil))
	if spa.Code != http.StatusOK || !strings.Contains(spa.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("SPA fallback failed: %d %q", spa.Code, spa.Header().Get("Content-Type"))
	}
}
