package api

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

func requestContext(c *gin.Context, candidates ...string) context.Context {
	tenantID := requestTenantID(c, candidates...)
	ctx := listingkit.WithTenantID(c.Request.Context(), tenantID)
	ctx = openaiclient.WithIdentity(ctx, openaiclient.Identity{TenantID: tenantID, UserID: requestUserID(c)})
	return listingkit.WithRequestRoles(ctx, requestRoles(c))
}

func detachedRequestContext(c *gin.Context, candidates ...string) context.Context {
	tenantID := requestTenantID(c, candidates...)
	ctx := listingkit.WithTenantID(context.Background(), tenantID)
	ctx = openaiclient.WithIdentity(ctx, openaiclient.Identity{TenantID: tenantID, UserID: requestUserID(c)})
	return listingkit.WithRequestRoles(ctx, requestRoles(c))
}

func requestTenantID(c *gin.Context, candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.Query("tenant_id")); value != "" {
		return value
	}
	return listingkit.DefaultTenantID
}

func requestUserID(c *gin.Context) string {
	for _, header := range []string{"X-User-ID", "X-User-Id", "X-User"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.Query("user_id")); value != "" {
		return value
	}
	return ""
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
