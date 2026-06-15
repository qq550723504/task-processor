package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

func RouteRequiresZitadelAuth(route httproute.Descriptor) bool {
	return route.Module == "listing-kit" ||
		route.Module == "listing-kit-admin" ||
		route.Module == "listing-kit-platform-admin" ||
		route.Module == "listing-kit-studio" ||
		route.Module == "shein-login" ||
		route.Module == "sds" ||
		route.Module == "sds-login"
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
	default:
		return ""
	}
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
		userRoles := roleHeaderValues(c.GetHeader("X-User-Roles"))
		if len(userRoles) == 0 {
			userRoles = roleHeaderValues(c.GetHeader("X-Zitadel-Roles"))
		}
		if authorizer.Authorize(c.GetHeader("X-User-ID"), userRoles, requiredPermission) {
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

func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {
	if identity == nil {
		return false, "ZITADEL identity is missing"
	}
	if len(cfg.AllowedTenantIDs) == 0 &&
		len(cfg.AllowedUserIDs) == 0 &&
		len(cfg.AllowedUsernames) == 0 &&
		len(cfg.AllowedRoles) == 0 {
		return false, "ZITADEL authorization is required but no allowlist is configured"
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.ResourceID), cfg.AllowedTenantIDs) {
		return true, ""
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.Subject, identity.UserID), cfg.AllowedUserIDs) {
		return true, ""
	}
	if valueInSet(firstNonEmptyZitadelValue(identity.Username), cfg.AllowedUsernames) {
		return true, ""
	}
	for _, role := range identity.Roles {
		if valueInSet(role, cfg.AllowedRoles) {
			return true, ""
		}
	}
	return false, "ZITADEL identity is not allowed to access ListingKit"
}
