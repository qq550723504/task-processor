package listingadmin

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
)

func requestIdentityContext(c *gin.Context) context.Context {
	if c == nil {
		return withRequestIdentity(nil, "", nil)
	}
	return withRequestIdentity(c.Request.Context(), requestUserID(c), requestRoles(c))
}

func requestScopedOwnerUserID(c *gin.Context) string {
	userID := requestUserID(c)
	if hasPlatformAdminRole(userID, requestRoles(c)) {
		return ""
	}
	return userID
}

func requestRoles(c *gin.Context) []string {
	if c == nil {
		return nil
	}
	seen := map[string]struct{}{}
	roles := make([]string, 0, 4)
	for _, header := range []string{"X-User-Roles", "X-Zitadel-Roles"} {
		for _, part := range strings.Split(c.GetHeader(header), ",") {
			role := strings.TrimSpace(part)
			if role == "" {
				continue
			}
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	return roles
}

func hasPlatformAdminRole(userID string, roles []string) bool {
	return authz.IsListingKitPlatformAdmin(userID, roles)
}
