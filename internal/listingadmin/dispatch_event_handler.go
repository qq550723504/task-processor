package listingadmin

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	dispatchEventDefaultWindow = time.Hour
	dispatchEventDefaultPage   = 1
	dispatchEventDefaultSize   = 50
	dispatchEventMaxPageSize   = 200
)

type DispatchEventHandler struct {
	repo DispatchEventRepository
	now  func() time.Time
}

func NewDispatchEventHandler(repo DispatchEventRepository) *DispatchEventHandler {
	return &DispatchEventHandler{repo: repo, now: time.Now}
}

func (h *DispatchEventHandler) GetDispatchEventSummary(c *gin.Context) {
	query, ok := h.dispatchEventQueryFromRequest(c, false)
	if !ok {
		return
	}
	summary, err := h.repo.GetDispatchEventSummary(requestIdentityContext(c), query)
	if err != nil {
		writeInternalHandlerError(c, "dispatch_event_summary_failed", err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *DispatchEventHandler) ListDispatchEvents(c *gin.Context) {
	query, ok := h.dispatchEventQueryFromRequest(c, true)
	if !ok {
		return
	}
	page, err := h.repo.ListDispatchEvents(requestIdentityContext(c), query)
	if err != nil {
		writeInternalHandlerError(c, "dispatch_event_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *DispatchEventHandler) dispatchEventQueryFromRequest(c *gin.Context, includePage bool) (DispatchEventQuery, bool) {
	scope := requestListScope(c)
	query := DispatchEventQuery{
		TenantID:   scope.TenantID,
		Platform:   strings.TrimSpace(c.Query("platform")),
		Action:     strings.TrimSpace(c.Query("action")),
		ReasonCode: strings.TrimSpace(c.Query("reasonCode")),
	}

	var ok bool
	if query.TenantID, ok = dispatchEventTenantID(c, scope.TenantID); !ok {
		return DispatchEventQuery{}, false
	}
	if query.StoreID, ok = queryInt64PtrStrict(c, "storeId", "invalid_store_id"); !ok {
		return DispatchEventQuery{}, false
	}
	if query.From, query.To, ok = dispatchEventWindow(c, h.currentTime()); !ok {
		return DispatchEventQuery{}, false
	}
	if includePage {
		if query.Page, query.PageSize, ok = dispatchEventPage(c); !ok {
			return DispatchEventQuery{}, false
		}
		query.Limit = query.PageSize
		query.Offset = (query.Page - 1) * query.PageSize
	}
	return query, true
}

func (h *DispatchEventHandler) currentTime() time.Time {
	if h != nil && h.now != nil {
		return h.now()
	}
	return time.Now()
}

func dispatchEventTenantID(c *gin.Context, fallback int64) (int64, bool) {
	if fallback <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return 0, false
	}
	value := strings.TrimSpace(c.Query("tenantId"))
	if value == "" {
		return fallback, true
	}
	parsed, ok := parseDispatchEventPositiveInt64(value)
	if !ok {
		writeValidationError(c, "invalid_tenant_id", errors.New("tenantId must be a positive integer"))
		return 0, false
	}
	if fallback > 0 && parsed != fallback {
		writeValidationError(c, "invalid_tenant_id", errors.New("tenantId must match request tenant scope"))
		return 0, false
	}
	return parsed, true
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
		writeValidationError(c, "invalid_time_window", errors.New("from must be before or equal to to"))
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
		writeValidationError(c, "invalid_"+key, errors.New(key+" must use RFC3339 format"))
		return time.Time{}, false
	}
	return parsed, true
}

func dispatchEventPage(c *gin.Context) (int, int, bool) {
	page, ok := queryPositiveInt(c, "page", dispatchEventDefaultPage, "invalid_page")
	if !ok {
		return 0, 0, false
	}
	pageSize, ok := queryPositiveInt(c, "page_size", dispatchEventDefaultSize, "invalid_page_size")
	if !ok {
		return 0, 0, false
	}
	if pageSize > dispatchEventMaxPageSize {
		writeValidationError(c, "invalid_page_size", errors.New("page_size must be less than or equal to 200"))
		return 0, 0, false
	}
	return page, pageSize, true
}

func parseDispatchEventPositiveInt64(value string) (int64, bool) {
	parsed := parseTenantID(value)
	return parsed, parsed > 0
}
