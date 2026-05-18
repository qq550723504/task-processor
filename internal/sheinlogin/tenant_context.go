package sheinlogin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/tenantbridge"
)

func requestTenantID(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, fmt.Errorf("tenant id is required")
	}

	for _, candidate := range []string{
		c.GetHeader("tenant-id"),
		c.GetHeader("X-Tenant-ID"),
		c.GetHeader("X-Tenant-Id"),
		c.GetHeader("X-Tenant"),
	} {
		if tenantID, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), candidate); err == nil && tenantID > 0 {
			return tenantID, nil
		}
	}

	if tenantValue, ok := parseTenantIDValueFromLoginUser(c.GetHeader("login-user")); ok {
		return tenantbridge.ResolveLegacyTenantID(c.Request.Context(), tenantValue)
	}

	return 0, fmt.Errorf("tenant id is required")
}

func parseTenantID(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	tenantID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, false
	}
	return tenantID, true
}

func parseTenantIDValueFromLoginUser(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return "", false
	}
	if tenantID := strings.TrimSpace(fmt.Sprint(payload["tenantId"])); tenantID != "" && tenantID != "<nil>" {
		return tenantID, true
	}
	if tenantID := strings.TrimSpace(fmt.Sprint(payload["tenant_id"])); tenantID != "" && tenantID != "<nil>" {
		return tenantID, true
	}
	return "", false
}
