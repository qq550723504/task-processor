package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
	sdshttpapi "task-processor/internal/sds/httpapi"
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

func appendSDSCatalogRouteDescriptors(routes []httproute.Descriptor, handlers ...sdshttpapi.HTTPRouteHandler) []httproute.Descriptor {
	var handler sdshttpapi.HTTPRouteHandler
	if len(handlers) > 0 {
		handler = handlers[0]
	}
	return sdshttpapi.AppendRouteDescriptors(routes, handler)
}
