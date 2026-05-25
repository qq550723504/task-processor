package listingadmin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/tenantbridge"
)

type handlerErrorRule struct {
	match     error
	status    int
	errorCode string
}

type listQueryScope struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		writeHandlerErrorResponse(c, http.StatusBadRequest, "invalid_request", err)
		return false
	}
	return true
}

func bindAndValidateJSON[T any](c *gin.Context, target *T, code string, prepare func(*T), validate func(*T) error) bool {
	if !bindJSON(c, target) {
		return false
	}
	if prepare != nil {
		prepare(target)
	}
	if validate != nil {
		if err := validate(target); err != nil {
			writeValidationError(c, code, err)
			return false
		}
	}
	return true
}

func requestPageParams(c *gin.Context) (page int, pageSize int) {
	return queryInt(c, "page", queryInt(c, "pageNo", 1)),
		queryInt(c, "page_size", queryInt(c, "pageSize", 20))
}

func requestListScope(c *gin.Context) listQueryScope {
	page, pageSize := requestPageParams(c)
	return listQueryScope{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Page:        page,
		PageSize:    pageSize,
	}
}

func requestTenantID(c *gin.Context) int64 {
	rawTenantID := ""
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			rawTenantID = value
			break
		}
	}
	if rawTenantID == "" {
		rawTenantID = strings.TrimSpace(c.Query("tenant_id"))
	}
	if rawTenantID == "" {
		return 0
	}
	value, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), rawTenantID)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func requestUserID(c *gin.Context) string {
	for _, header := range []string{"X-User-ID", "X-User-Id", "X-User"} {
		if userID := requestUserIDHeader(c.GetHeader(header)); userID != "" {
			return userID
		}
	}
	return requestUserIDHeader(c.Query("user_id"))
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id", "message": "id must be a positive integer"})
		return 0, false
	}
	return id, true
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryBoolPtr(c *gin.Context, key string) *bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func queryInt64Ptr(c *gin.Context, key string) *int64 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed := parseTenantID(value)
	if parsed <= 0 {
		return nil
	}
	return &parsed
}

func queryInt16Ptr(c *gin.Context, key string) *int16 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return nil
	}
	out := int16(parsed)
	return &out
}

func queryIntPtr(c *gin.Context, key string) *int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func writeHandlerErrorResponse(c *gin.Context, status int, code string, err error) {
	c.JSON(status, gin.H{"error": code, "message": err.Error()})
}

func writeValidationError(c *gin.Context, code string, err error) {
	writeHandlerErrorResponse(c, http.StatusBadRequest, code, err)
}

func writeInternalHandlerError(c *gin.Context, code string, err error) {
	writeHandlerErrorResponse(c, http.StatusInternalServerError, code, err)
}

func writeMappedHandlerError(c *gin.Context, err error, fallbackCode string, rules ...handlerErrorRule) {
	for _, rule := range rules {
		if errors.Is(err, rule.match) {
			writeHandlerErrorResponse(c, rule.status, rule.errorCode, err)
			return
		}
	}
	writeHandlerErrorResponse(c, http.StatusInternalServerError, fallbackCode, err)
}
