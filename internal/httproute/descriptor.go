package httproute

import "github.com/gin-gonic/gin"

type AuthPolicy string

const (
	AuthPolicyUnspecified      AuthPolicy = ""
	AuthPolicyPublic           AuthPolicy = "public"
	AuthPolicyVerifiedIdentity AuthPolicy = "verified_identity"
)

// Descriptor describes a single HTTP route registration.
type Descriptor struct {
	Method     string
	Path       string
	Module     string
	Permission string
	AuthPolicy AuthPolicy
	Handler    gin.HandlerFunc
}
