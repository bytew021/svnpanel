package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/internal/svncfg"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listGroups(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.ListGroups()
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

type groupRequest struct {
	revisionsRequest
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Remark  string   `json:"remark"`
}

func createGroup(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request groupRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.CreateGroup(
			request.revisionsRequest.value(), request.Name, request.Remark, request.Members, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.Created(c, result)
	}
}

type groupMembersRequest struct {
	revisionsRequest
	Members []string `json:"members"`
	Remark  *string  `json:"remark"`
}

type groupPermissionsRequest struct {
	revisionsRequest
	Permissions []svncfg.GroupPermission `json:"permissions"`
}

func setGroupMembers(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request groupMembersRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.SetGroupMembers(
			request.revisionsRequest.value(), c.Param("name"), request.Members, request.Remark, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

func setGroupPermissions(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request groupPermissionsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.SetGroupPermissions(
			request.revisionsRequest.value(), c.Param("name"), request.Permissions, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type deleteGroupRequest struct {
	revisionsRequest
}

func deleteGroup(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deleteGroupRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.DeleteGroup(request.revisionsRequest.value(), c.Param("name"), c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
