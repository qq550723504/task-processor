package api

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/tenantbridge"
)

func parseSheinScopedRequest(c *gin.Context) (storeID int64, tenantID int64, ctx context.Context, ok bool) {
	storeID, err := parseSheinInt64Param(c, "store_id")
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid_request", "message": err.Error()})
		return 0, 0, nil, false
	}
	tenantID, err = parseSheinTenantID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid_request", "message": err.Error()})
		return 0, 0, nil, false
	}
	return storeID, tenantID, requestContext(c, strconv.FormatInt(tenantID, 10)), true
}

func parseSheinTenantID(c *gin.Context) (int64, error) {
	value := strings.TrimSpace(requestTenantID(c))
	if value == "" || value == listingkit.DefaultTenantID {
		return 0, errors.New("numeric tenant_id is required")
	}
	tenantID, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), value)
	if err != nil || tenantID <= 0 {
		return 0, errors.New("numeric tenant_id is required")
	}
	return tenantID, nil
}

func parseSheinInt64Param(c *gin.Context, name string) (int64, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return 0, errors.New(name + " is required")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return parsed, nil
}

func parseOptionalBoolQuery(value string) (*bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return nil, errors.New("invalid is_active")
	}
	return &parsed, nil
}
