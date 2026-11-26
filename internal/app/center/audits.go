package center

import (
	"net/http"
	"strconv"
	"time"

	"svnpanel/internal/audit"
	"svnpanel/internal/service"
	"svnpanel/pkg/resp"

	"github.com/gin-gonic/gin"
)

func listAudits(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter := audit.Filter{
			Action:  c.Query("action"),
			Outcome: c.Query("outcome"),
			Keyword: c.Query("keyword"),
			Page:    parsePositive(c.Query("page"), 1),
			Size:    parsePositive(c.Query("pageSize"), 50),
		}
		if from := c.Query("from"); from != "" {
			value, err := parseDate(from, false)
			if err != nil {
				resp.Fail(c, http.StatusBadRequest, "validation_failed", "from 时间格式错误", nil)
				return
			}
			filter.From = &value
		}
		if to := c.Query("to"); to != "" {
			value, err := parseDate(to, true)
			if err != nil {
				resp.Fail(c, http.StatusBadRequest, "validation_failed", "to 时间格式错误", nil)
				return
			}
			filter.To = &value
		}
		result, err := svc.QueryAudit(filter)
		if err != nil {
			respondError(c, err)
			return
		}
		resp.OK(c, result)
	}
}

func parsePositive(value string, fallback int) int {
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return fallback
	}
	return number
}

func parseDate(value string, endOfDay bool) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		if endOfDay {
			parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		}
		return parsed, nil
	}
	return time.Time{}, &time.ParseError{Layout: "RFC3339 或 2006-01-02", Value: value}
}
