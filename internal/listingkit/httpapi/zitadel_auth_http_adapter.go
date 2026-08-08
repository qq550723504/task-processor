package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WrapZitadelAuthMiddleware adapts the ListingKit Gin auth middleware to a net/http handler.
func WrapZitadelAuthMiddleware(next http.Handler, middleware gin.HandlerFunc) http.Handler {
	if middleware == nil {
		return next
	}
	router := gin.New()
	router.Use(middleware)
	router.Any("/*path", func(c *gin.Context) {
		next.ServeHTTP(c.Writer, c.Request)
	})
	return router
}
