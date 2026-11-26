package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

type departmentRequest struct {
	revisionsRequest
	Name string `json:"name"`
}

func addDepartment(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request departmentRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.AddDepartment(request.revisionsRequest.value(), request.Name, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.Created(c, result)
	}
}

func renameDepartment(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request departmentRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.RenameDepartment(
			request.revisionsRequest.value(), c.Param("name"), request.Name, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type deleteDepartmentRequest struct {
	revisionsRequest
}

func deleteDepartment(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deleteDepartmentRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.DeleteDepartment(
			request.revisionsRequest.value(), c.Param("name"), c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
