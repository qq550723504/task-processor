package httproute

import "github.com/gin-gonic/gin"

// Descriptor describes a single HTTP route registration.
type Descriptor struct {
	Method     string
	Path       string
	Module     string
	Permission string
	Handler    gin.HandlerFunc
}
