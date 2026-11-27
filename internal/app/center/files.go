package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listRawFiles(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.GetRawFiles()
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

type updateRawFileRequest struct {
	revisionsRequest
	Content string `json:"content"`
}

func updateRawFile(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request updateRawFileRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.UpdateRawFile(
			request.revisionsRequest.value(), c.Param("name"), request.Content, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
