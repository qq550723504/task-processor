package httpapi

import "github.com/gin-gonic/gin"

type sheinPODImageLookupRouteHandler interface {
	LookupSheinPODImages(c *gin.Context)
}
