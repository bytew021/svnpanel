package center

import (
	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func getSvnserveInfo(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := svc.GetSvnserveInfo(c.Request.Context())
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}
