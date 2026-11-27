package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listUsers(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.ListUsers()
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

type createUserRequest struct {
	revisionsRequest
	Name       string   `json:"name"`
	Password   string   `json:"password"`
	Remark     string   `json:"remark"`
	Department string   `json:"department"`
	Groups     []string `json:"groups"`
}

func createUser(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request createUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.CreateUser(
			request.revisionsRequest.value(), request.Name, request.Password,
			request.Remark, request.Department, request.Groups, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.Created(c, result)
	}
}

type userMetadataRequest struct {
	revisionsRequest
	Remark     string `json:"remark"`
	Department string `json:"department"`
}

func setUserMetadata(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request userMetadataRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.SetUserMetadata(
			request.revisionsRequest.value(), c.Param("name"), request.Remark, request.Department, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type patchUserRequest struct {
	revisionsRequest
	Disabled bool `json:"disabled"`
}

func patchUser(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request patchUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.SetUserDisabled(request.revisionsRequest.value(), c.Param("name"), request.Disabled, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type passwordRequest struct {
	revisionsRequest
	Password string `json:"password"`
}

func resetPassword(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request passwordRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.ResetPassword(request.revisionsRequest.value(), c.Param("name"), request.Password, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

func revealPassword(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.RevealPassword(currentSession(c), c.Param("name"), c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		resp.OK(c, result)
	}
}

type deleteUserRequest struct {
	revisionsRequest
}

type batchUsersRequest struct {
	revisionsRequest
	Names      []string `json:"names"`
	Action     string   `json:"action"`
	Password   string   `json:"password"`
	Department string   `json:"department"`
}

func batchUsers(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request batchUsersRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.BatchUsers(
			request.revisionsRequest.value(), request.Names, request.Action,
			request.Password, request.Department, c.ClientIP(),
		)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

func deleteUser(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deleteUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		result, err := svc.DeleteUser(request.revisionsRequest.value(), c.Param("name"), c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
