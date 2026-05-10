package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

func requestContext(c *gin.Context, candidates ...string) context.Context {
	tenantID := requestTenantID(c, candidates...)
	ctx := listingkit.WithTenantID(c.Request.Context(), tenantID)
	return openaiclient.WithIdentity(ctx, openaiclient.Identity{TenantID: tenantID, UserID: requestUserID(c)})
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
	if loginUser := requestLoginUser(c); loginUser != nil && strings.TrimSpace(loginUser.TenantID) != "" {
		return loginUser.TenantID
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
	if loginUser := requestLoginUser(c); loginUser != nil && strings.TrimSpace(loginUser.ID) != "" {
		return loginUser.ID
	}
	if value := strings.TrimSpace(c.Query("user_id")); value != "" {
		return value
	}
	return ""
}

type yudaoLoginUserHeader struct {
	ID       string
	TenantID string
}

func requestLoginUser(c *gin.Context) *yudaoLoginUserHeader {
	value := strings.TrimSpace(c.GetHeader("login-user"))
	if value == "" {
		return nil
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil
	}
	user := &yudaoLoginUserHeader{
		ID:       stringifyJSONIdentityValue(payload["id"]),
		TenantID: stringifyJSONIdentityValue(payload["tenantId"]),
	}
	if user.TenantID == "" {
		user.TenantID = stringifyJSONIdentityValue(payload["tenant_id"])
	}
	return user
}

func stringifyJSONIdentityValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return strings.TrimSpace(fmt.Sprint(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
