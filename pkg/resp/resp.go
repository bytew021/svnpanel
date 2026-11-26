package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Fail(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, Envelope{Error: &Error{Code: code, Message: message, Details: details}})
}
