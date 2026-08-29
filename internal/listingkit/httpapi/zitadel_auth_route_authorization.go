package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

func RouteRequiresZitadelAuth(route httproute.Descriptor) bool {
	switch route.AuthPolicy {
	case httproute.AuthPolicyVerifiedIdentity:
		return true
	case httproute.AuthPolicyPublic:
		return false
	}
	if route.Method == http.MethodGet && (route.Path == "/api/v1/shein-login/health" || route.Path == "/api/v1/sds-login/health") {
		return false
	}
	if route.Module == "listing-kit-zitadel-sms-webhook" {
		// ZITADEL does not send a user Bearer token. Only this exact callback
		// authenticates with the fresh raw-body HMAC validated by its handler.
		return route.Method != http.MethodPost || route.Path != "/api/v1/listing-kits/integrations/zitadel/sms"
	}
	return route.Module == "listing-kit" ||
		route.Module == "listing-kit-admin" ||
		route.Module == "listing-kit-platform-admin" ||
		route.Module == "listing-kit-prompts" ||
		route.Module == "listing-kit-studio" ||
		route.Module == "shein-login" ||
		route.Module == "sds" ||
		route.Module == "sds-login" ||
		route.Module == "product-sourcing" ||
		route.Module == "local-agent" ||
		route.Module == "crawler-1688" ||
		route.Module == "image-agent" ||
		route.Module == "images"
}

func listingKitRouteRequiredPermission(route httproute.Descriptor) string {
	if value := strings.TrimSpace(route.Permission); value != "" {
		return value
	}
	switch route.Module {
	case "listing-kit-admin":
		if route.Method == http.MethodGet {
			return authz.PermissionListingKitAdminRead
		}
		return authz.PermissionListingKitAdminWrite
	case "listing-kit-platform-admin":
		return authz.PermissionListingKitPlatformAdm
	case "listing-kit-prompts":
		if route.Method == http.MethodPut || route.Method == http.MethodPatch {
			return authz.PermissionListingKitPromptWrite
		}
	case "sds-login":
		return authz.PermissionListingKitPlatformAdm
	default:
		return ""
	}
	return ""
}

func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {
	requiredPermission := listingKitRouteRequiredPermission(route)
	if requiredPermission == "" {
		return nil
	}
	runtimeCfg := currentListingKitZitadelRuntimeConfig()
	var authorizer *authz.ListingKitAuthorizer
	if runtimeCfg != nil {
		authorizer = runtimeCfg.Authorizer
	}
	if authorizer == nil {
		var err error
		authorizer, err = authz.NewListingKitAuthorizer(nil, nil)
		if err != nil {
			return func(c *gin.Context) {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "listingkit_authorization_unavailable",
					"message": "ListingKit authorization is not available",
				})
			}
		}
	}
	return func(c *gin.Context) {
		identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "listingkit_permission_denied",
				"message": "ZITADEL identity is required to access this ListingKit route",
			})
			return
		}
		if authorizer.Authorize(identity.UserID, identity.Roles, requiredPermission) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":               "listingkit_permission_denied",
			"message":             "ZITADEL identity is not allowed to access this ListingKit route",
			"required_permission": requiredPermission,
		})
	}
}

func roleHeaderValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	roles := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		role := strings.TrimSpace(item)
		if role != "" {
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	return roles
}
