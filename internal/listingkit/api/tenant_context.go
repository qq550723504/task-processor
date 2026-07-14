package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

func requestContext(c *gin.Context, candidates ...string) context.Context {
	tenantID := requestTenantID(c, candidates...)
	ctx := listingkit.WithTenantID(c.Request.Context(), tenantID)
	ctx = openaiclient.WithIdentity(ctx, openaiclient.Identity{TenantID: tenantID, UserID: requestUserID(c)})
	ctx = listingkit.WithRequestRoles(ctx, requestRoles(c))
	return listingkit.WithRequestTrace(ctx, requestTrace(c))
}

func detachedRequestContext(c *gin.Context, candidates ...string) context.Context {
	tenantID := requestTenantID(c, candidates...)
	ctx := listingkit.WithTenantID(context.Background(), tenantID)
	ctx = openaiclient.WithIdentity(ctx, openaiclient.Identity{TenantID: tenantID, UserID: requestUserID(c)})
	ctx = listingkit.WithRequestRoles(ctx, requestRoles(c))
	return listingkit.WithRequestTrace(ctx, requestTrace(c))
}

func requestTenantID(c *gin.Context, candidates ...string) string {
	if tenantID, ok := requestExplicitTenantID(c, candidates...); ok {
		return tenantID
	}
	return listingkit.DefaultTenantID
}

func requestExplicitTenantID(c *gin.Context, candidates ...string) (string, bool) {
	if identity, ok := authenticatedIdentity(c); ok {
		return identity.TenantID, true
	}
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed, true
		}
	}
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value, true
		}
	}
	if value := strings.TrimSpace(c.Query("tenant_id")); value != "" {
		return value, true
	}
	return "", false
}

func requireExplicitRequestContext(c *gin.Context, candidates ...string) (context.Context, bool) {
	tenantID, ok := requestExplicitTenantID(c, candidates...)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "tenant_id is required"})
		return nil, false
	}
	return requestContext(c, tenantID), true
}

func requestUserID(c *gin.Context) string {
	if identity, ok := authenticatedIdentity(c); ok {
		return identity.UserID
	}
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
	if identity, ok := authenticatedIdentity(c); ok {
		return append([]string(nil), identity.Roles...)
	}
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

func authenticatedIdentity(c *gin.Context) (listingkit.AuthenticatedIdentity, bool) {
	if c == nil || c.Request == nil {
		return listingkit.AuthenticatedIdentity{}, false
	}
	return listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
}

func requestTrace(c *gin.Context) listingkit.RequestTrace {
	if c == nil {
		return listingkit.RequestTrace{}
	}
	return listingkit.ParseRequestTrace(
		c.GetHeader("X-ListingKit-Batch-Run-Id"),
		c.GetHeader("X-ListingKit-Batch-Id"),
		c.GetHeader("X-ListingKit-Studio-Session-Id"),
		c.GetHeader("X-ListingKit-Queue-Mode"),
		c.GetHeader("X-ListingKit-Queue-Index"),
		c.GetHeader("X-ListingKit-Queue-Total"),
	)
}
