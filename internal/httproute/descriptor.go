package httproute

import "github.com/gin-gonic/gin"

type AuthPolicy string

type OrganizationAccessPolicy string

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
	Method                   string
	Path                     string
	Module                   string
	Permission               string
	AuthPolicy               AuthPolicy
	OrganizationAccessPolicy OrganizationAccessPolicy
	Handler                  gin.HandlerFunc
}
