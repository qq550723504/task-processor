package listingadmin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type handlerErrorRule struct {
	match     error
	status    int
	errorCode string
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		writeHandlerErrorResponse(c, http.StatusBadRequest, "invalid_request", err)
		return false
	}
	return true
}

func writeHandlerErrorResponse(c *gin.Context, status int, code string, err error) {
	c.JSON(status, gin.H{"error": code, "message": err.Error()})
}

func writeMappedHandlerError(c *gin.Context, err error, fallbackCode string, rules ...handlerErrorRule) {
	for _, rule := range rules {
		if errors.Is(err, rule.match) {
			writeHandlerErrorResponse(c, rule.status, rule.errorCode, err)
			return
		}
	}
	writeHandlerErrorResponse(c, http.StatusInternalServerError, fallbackCode, err)
}
