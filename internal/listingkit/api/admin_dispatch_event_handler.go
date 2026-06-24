package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingadmin"
	"task-processor/internal/tenantbridge"
)

const (
	dispatchEventDefaultWindow = time.Hour
	dispatchEventDefaultPage   = 1
	dispatchEventDefaultSize   = 50
	dispatchEventMaxPageSize   = 200
)

func (h *handler) getAdminDispatchEventSummary(c *gin.Context) {
	query, ok := h.dispatchEventQueryFromRequest(c, false)
	if !ok {
		return
	}
	summary, err := h.dispatchEventRepository.GetDispatchEventSummary(requestContext(c, strconv.FormatInt(query.TenantID, 10)), query)
	if err != nil {
		writeAdminDispatchEventInternalError(c, "dispatch_event_summary_failed", err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *handler) listAdminDispatchEvents(c *gin.Context) {
	query, ok := h.dispatchEventQueryFromRequest(c, true)
	if !ok {
		return
	}
	page, err := h.dispatchEventRepository.ListDispatchEvents(requestContext(c, strconv.FormatInt(query.TenantID, 10)), query)
	if err != nil {
		writeAdminDispatchEventInternalError(c, "dispatch_event_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) dispatchEventQueryFromRequest(c *gin.Context, includePage bool) (listingadmin.DispatchEventQuery, bool) {
	tenantID, ok := dispatchEventTenantID(c)
	if !ok {
		return listingadmin.DispatchEventQuery{}, false
	}
	query := listingadmin.DispatchEventQuery{
		TenantID:   tenantID,
		Platform:   strings.TrimSpace(c.Query("platform")),
		Action:     strings.TrimSpace(c.Query("action")),
		ReasonCode: strings.TrimSpace(c.Query("reasonCode")),
	}
	if query.StoreID, ok = dispatchEventInt64Ptr(c, "storeId", "invalid_store_id"); !ok {
		return listingadmin.DispatchEventQuery{}, false
	}
	if query.From, query.To, ok = dispatchEventWindow(c, time.Now()); !ok {
		return listingadmin.DispatchEventQuery{}, false
	}
	if includePage {
		if query.Page, query.PageSize, ok = dispatchEventPage(c); !ok {
			return listingadmin.DispatchEventQuery{}, false
		}
		query.Limit = query.PageSize
		query.Offset = (query.Page - 1) * query.PageSize
	}
	return query, true
}

func dispatchEventTenantID(c *gin.Context) (int64, bool) {
	rawTenantID := dispatchEventRequestTenantID(c)
	if strings.TrimSpace(rawTenantID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return 0, false
	}
	resolvedTenantID, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), rawTenantID)
	if err != nil || resolvedTenantID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return 0, false
	}

	value := strings.TrimSpace(c.Query("tenantId"))
	if value == "" {
		return resolvedTenantID, true
	}
	parsed, ok := parseDispatchEventPositiveInt64(value)
	if !ok {
		writeAdminDispatchEventValidationError(c, "invalid_tenant_id", errors.New("tenantId must be a positive integer"))
		return 0, false
	}
	if parsed != resolvedTenantID {
		writeAdminDispatchEventValidationError(c, "invalid_tenant_id", errors.New("tenantId must match request tenant scope"))
		return 0, false
	}
	return parsed, true
}

func dispatchEventRequestTenantID(c *gin.Context) string {
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	return ""
}

func dispatchEventWindow(c *gin.Context, now time.Time) (time.Time, time.Time, bool) {
	from, ok := dispatchEventTime(c, "from")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	to, ok := dispatchEventTime(c, "to")
	if !ok {
		return time.Time{}, time.Time{}, false
	}

	switch {
	case from.IsZero() && to.IsZero():
		to = now
		from = now.Add(-dispatchEventDefaultWindow)
	case from.IsZero():
		from = to.Add(-dispatchEventDefaultWindow)
	case to.IsZero():
		to = from.Add(dispatchEventDefaultWindow)
	}
	if from.After(to) {
		writeAdminDispatchEventValidationError(c, "invalid_time_window", errors.New("from must be before or equal to to"))
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func dispatchEventTime(c *gin.Context, key string) (time.Time, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		writeAdminDispatchEventValidationError(c, "invalid_"+key, errors.New(key+" must use RFC3339 format"))
		return time.Time{}, false
	}
	return parsed, true
}

func dispatchEventPage(c *gin.Context) (int, int, bool) {
	page, ok := dispatchEventPositiveInt(c, "page", dispatchEventDefaultPage, "invalid_page")
	if !ok {
		return 0, 0, false
	}
	pageSize, ok := dispatchEventPositiveInt(c, "page_size", dispatchEventDefaultSize, "invalid_page_size")
	if !ok {
		return 0, 0, false
	}
	if pageSize > dispatchEventMaxPageSize {
		writeAdminDispatchEventValidationError(c, "invalid_page_size", errors.New("page_size must be less than or equal to 200"))
		return 0, 0, false
	}
	return page, pageSize, true
}

func dispatchEventPositiveInt(c *gin.Context, key string, fallback int, errorCode string) (int, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		writeAdminDispatchEventValidationError(c, errorCode, errors.New(key+" must be a positive integer"))
		return 0, false
	}
	return parsed, true
}

func dispatchEventInt64Ptr(c *gin.Context, key string, errorCode string) (*int64, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil, true
	}
	parsed, ok := parseDispatchEventPositiveInt64(value)
	if !ok {
		writeAdminDispatchEventValidationError(c, errorCode, errors.New(key+" must be a positive integer"))
		return nil, false
	}
	return &parsed, true
}

func parseDispatchEventPositiveInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil && parsed > 0
}

func writeAdminDispatchEventValidationError(c *gin.Context, code string, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": code, "message": err.Error()})
}

func writeAdminDispatchEventInternalError(c *gin.Context, code string, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
