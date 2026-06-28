package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
)

func buildCoreRouteDescriptors() []httproute.Descriptor {
	return []httproute.Descriptor{
		{
			Method: http.MethodGet,
			Path:   "/health",
			Module: "system",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			},
		},
	}
}
