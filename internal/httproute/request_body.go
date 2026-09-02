package httproute

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// WithRequestBodyReadTimeout closes a request body that a route handler has
// been reading for too long. Closing the body interrupts a blocked network
// read while keeping the timeout scoped to routes that accept a body.
func WithRequestBodyReadTimeout(timeout time.Duration, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || timeout <= 0 {
			handler(c)
			return
		}
		body := c.Request.Body
		timer := time.AfterFunc(timeout, func() { _ = body.Close() })
		defer timer.Stop()
		handler(c)
	}
}

// WithRequestBodyReadTimeoutResolver applies the same boundary to a route
// resolver that reads the body before the final Gin handler is invoked.
func WithRequestBodyReadTimeoutResolver(timeout time.Duration, resolver OrganizationTargetResolver) OrganizationTargetResolver {
	return func(request *http.Request) (string, error) {
		if request == nil || request.Body == nil || timeout <= 0 {
			return resolver(request)
		}
		body := request.Body
		timer := time.AfterFunc(timeout, func() { _ = body.Close() })
		defer timer.Stop()
		return resolver(request)
	}
}
