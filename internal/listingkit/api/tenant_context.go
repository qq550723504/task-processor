package api

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func requestContext(c *gin.Context, candidates ...string) context.Context {
	return listingkit.WithTenantID(c.Request.Context(), requestTenantID(c, candidates...))
}

func requestTenantID(c *gin.Context, candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.Query("tenant_id")); value != "" {
		return value
	}
	return listingkit.DefaultTenantID
}
