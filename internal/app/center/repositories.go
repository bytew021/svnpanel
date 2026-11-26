package center

import (
	"net/http"

	"svnpanel/internal/service"
	"svnpanel/internal/svncfg"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listRepositories(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.ListRepositories(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, gin.H{"items": result})
	}
}

func listPaths(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		parent := c.DefaultQuery("parent", "/")
		result, err := svc.ListPaths(c.Request.Context(), c.Param("repo"), parent)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, gin.H{"parent": svncfg.NormalizePath(parent), "items": result})
	}
}

func getACL(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := svc.GetACL(c.Param("repo"), c.DefaultQuery("path", "/"), c.Query("user"))
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type aclRequest struct {
	revisionsRequest
	Path       string              `json:"path"`
	Kind       svncfg.IdentityKind `json:"kind"`
	Name       string              `json:"name"`
	Permission svncfg.Permission   `json:"permission"`
}

func setACL(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request aclRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		identity := svncfg.ACLIdentity{Kind: request.Kind, Name: request.Name}
		result, err := svc.SetACL(request.revisionsRequest.value(), c.Param("repo"), request.Path, identity, request.Permission, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

type deleteACLRequest struct {
	revisionsRequest
	Path string              `json:"path"`
	Kind svncfg.IdentityKind `json:"kind"`
	Name string              `json:"name"`
}

func deleteACL(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deleteACLRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			resp.Fail(c, http.StatusBadRequest, "bad_request", "请求格式错误", nil)
			return
		}
		identity := svncfg.ACLIdentity{Kind: request.Kind, Name: request.Name}
		result, err := svc.DeleteACL(request.revisionsRequest.value(), c.Param("repo"), request.Path, identity, c.ClientIP())
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}
