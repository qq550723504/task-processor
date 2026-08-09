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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/health" || r.URL.Path == "/ready" {
			next.ServeHTTP(w, r)
			return
		}
		router.ServeHTTP(w, r)
	})
}
