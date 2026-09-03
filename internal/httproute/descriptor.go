package httproute

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthPolicy string

type OrganizationAccessPolicy string

// OrganizationTargetResolver validates an untrusted request candidate before
// the live organization resolver performs an authorization lookup.
type OrganizationTargetResolver func(*http.Request) (string, error)

const (
	AuthPolicyUnspecified      AuthPolicy = ""
	AuthPolicyPublic           AuthPolicy = "public"
	AuthPolicyVerifiedIdentity AuthPolicy = "verified_identity"
)

const (
	OrganizationAccessPolicyNone        OrganizationAccessPolicy = "none"
	OrganizationAccessPolicyContextRead OrganizationAccessPolicy = "context_read"
	OrganizationAccessPolicyCachedRead  OrganizationAccessPolicy = "cached_read"
	OrganizationAccessPolicyLiveWrite   OrganizationAccessPolicy = "live_write"
	OrganizationAccessPolicyLiveSwitch  OrganizationAccessPolicy = "live_switch"
)

// Descriptor describes a single HTTP route registration.
type Descriptor struct {
	Method                     string
	Path                       string
	Module                     string
	Permission                 string
	AuthPolicy                 AuthPolicy
	OrganizationAccessPolicy   OrganizationAccessPolicy
	OrganizationTargetResolver OrganizationTargetResolver
	Handler                    gin.HandlerFunc
}
