package sheinlogin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/listingkit"
	"task-processor/internal/tenantbridge"
)

const visitTenantIDHeader = "X-Shein-Login-Visit-Tenant-ID"

var errVisitTenantAccessDenied = errors.New("platform administrator role is required to manage another tenant's SHEIN login")

func requestTenantID(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, fmt.Errorf("tenant id is required")
	}

	if visitTenantID := strings.TrimSpace(c.GetHeader(visitTenantIDHeader)); visitTenantID != "" {
		identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
		if !ok || !authz.IsListingKitPlatformAdmin(identity.UserID, identity.Roles) {
			return 0, errVisitTenantAccessDenied
		}
		return tenantbridge.ResolveLegacyTenantID(c.Request.Context(), visitTenantID)
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
