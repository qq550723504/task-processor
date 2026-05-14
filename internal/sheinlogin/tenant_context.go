package sheinlogin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func requestTenantID(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, fmt.Errorf("tenant id is required")
	}

	candidates := []string{
		c.GetHeader("tenant-id"),
		c.GetHeader("X-Tenant-ID"),
		c.GetHeader("X-Tenant-Id"),
		c.GetHeader("X-Tenant"),
	}
	for _, candidate := range candidates {
		if tenantID, ok := parseTenantID(candidate); ok {
			return tenantID, nil
		}
	}

	if tenantID, ok := parseTenantIDFromLoginUser(c.GetHeader("login-user")); ok {
		return tenantID, nil
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

func parseTenantIDFromLoginUser(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return 0, false
	}
	if tenantID, ok := parseTenantID(fmt.Sprint(payload["tenantId"])); ok {
		return tenantID, true
	}
	if tenantID, ok := parseTenantID(fmt.Sprint(payload["tenant_id"])); ok {
		return tenantID, true
	}
	return 0, false
}
