package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listSnapshots(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.ListSnapshots()
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

func getSnapshot(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.GetSnapshot(c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

type restoreSnapshotRequest struct {
	revisionsRequest
}

func restoreSnapshot(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request restoreSnapshotRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.RestoreSnapshot(
			request.revisionsRequest.value(), c.Param("id"), c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
