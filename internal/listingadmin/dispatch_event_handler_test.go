package listingadmin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeDispatchEventRepository struct {
	summaryQuery DispatchEventQuery
	listQuery    DispatchEventQuery
	summaryErr   error
	listErr      error
}

func (f *fakeDispatchEventRepository) GetDispatchEventSummary(_ context.Context, query DispatchEventQuery) (*DispatchEventSummary, error) {
	f.summaryQuery = query
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	return &DispatchEventSummary{
		Window:     DispatchEventWindow{From: query.From, To: query.To},
		Total:      3,
		Dispatched: 1,
		Skipped:    2,
		ReasonCounts: []DispatchEventReasonCount{
			{ReasonCode: "no_capacity", Action: "skipped", Count: 2},
		},
	}, nil
}

func (f *fakeDispatchEventRepository) ListDispatchEvents(_ context.Context, query DispatchEventQuery) (*DispatchEventPage, error) {
	f.listQuery = query
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &DispatchEventPage{
		Items: []DispatchEventItem{
			{ID: 1, TaskID: 101, TenantID: query.TenantID, StoreID: 976, Action: "skipped", ReasonCode: "no_capacity"},
		},
		Total:    1,
		Page:     query.Page,
		PageSize: query.PageSize,
		Limit:    query.Limit,
		Offset:   query.Offset,
	}, nil
}

func TestDispatchEventHandlerSummaryUsesDefaultWindowAndRequestTenant(t *testing.T) {
	repo := &fakeDispatchEventRepository{}
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	router := newDispatchEventHandlerTestRouter(repo, now)

	req := httptest.NewRequest(http.MethodGet, "/dispatch-events/summary", nil)
	req.Header.Set("X-Tenant-ID", "246")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /dispatch-events/summary = %d, body=%s", resp.Code, resp.Body.String())
	}
	if repo.summaryQuery.TenantID != 246 {
		t.Fatalf("tenant id = %d, want request tenant 246", repo.summaryQuery.TenantID)
	}
	if !repo.summaryQuery.From.Equal(now.Add(-time.Hour)) || !repo.summaryQuery.To.Equal(now) {
		t.Fatalf("window = %s - %s, want previous hour ending now", repo.summaryQuery.From, repo.summaryQuery.To)
	}
}

func TestDispatchEventHandlerListParsesFiltersAndPagination(t *testing.T) {
	repo := &fakeDispatchEventRepository{}
	now := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	router := newDispatchEventHandlerTestRouter(repo, now)

	req := httptest.NewRequest(http.MethodGet, "/dispatch-events?platform=shein&tenantId=246&storeId=976&action=skipped&reasonCode=no_capacity&from=2026-06-24T14:00:00Z&to=2026-06-24T15:00:00Z&page=3&page_size=25", nil)
	req.Header.Set("X-Tenant-ID", "246")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /dispatch-events = %d, body=%s", resp.Code, resp.Body.String())
	}
	query := repo.listQuery
	if query.TenantID != 246 {
		t.Fatalf("tenant id = %d, want request tenant 246", query.TenantID)
	}
	if query.Platform != "shein" || query.Action != "skipped" || query.ReasonCode != "no_capacity" {
		t.Fatalf("filters = platform:%q action:%q reason:%q", query.Platform, query.Action, query.ReasonCode)
	}
	if query.StoreID == nil || *query.StoreID != 976 {
		t.Fatalf("store id = %+v, want 976", query.StoreID)
	}
	if !query.From.Equal(time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)) || !query.To.Equal(time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("window = %s - %s", query.From, query.To)
	}
	if query.Page != 3 || query.PageSize != 25 || query.Limit != 25 || query.Offset != 50 {
		t.Fatalf("pagination = page:%d pageSize:%d limit:%d offset:%d", query.Page, query.PageSize, query.Limit, query.Offset)
	}
}

func TestDispatchEventHandlerCompletesSingleSidedWindows(t *testing.T) {
	now := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		path     string
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			name:     "to only",
			path:     "/dispatch-events/summary?to=2026-06-24T15:00:00Z",
			wantFrom: time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC),
			wantTo:   time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC),
		},
		{
			name:     "from only",
			path:     "/dispatch-events/summary?from=2026-06-24T14:30:00Z",
			wantFrom: time.Date(2026, 6, 24, 14, 30, 0, 0, time.UTC),
			wantTo:   time.Date(2026, 6, 24, 15, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDispatchEventRepository{}
			router := newDispatchEventHandlerTestRouter(repo, now)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, body=%s", tt.path, resp.Code, resp.Body.String())
			}
			if !repo.summaryQuery.From.Equal(tt.wantFrom) || !repo.summaryQuery.To.Equal(tt.wantTo) {
				t.Fatalf("window = %s - %s, want %s - %s", repo.summaryQuery.From, repo.summaryQuery.To, tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestDispatchEventHandlerRejectsInvalidQueryParams(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		errorCode string
	}{
		{name: "from", path: "/dispatch-events?from=2026-06-24", errorCode: "invalid_from"},
		{name: "to", path: "/dispatch-events?to=bad-time", errorCode: "invalid_to"},
		{name: "tenantId", path: "/dispatch-events?tenantId=abc", errorCode: "invalid_tenant_id"},
		{name: "tenant scope mismatch", path: "/dispatch-events?tenantId=357", errorCode: "invalid_tenant_id"},
		{name: "storeId", path: "/dispatch-events?storeId=abc", errorCode: "invalid_store_id"},
		{name: "page", path: "/dispatch-events?page=0", errorCode: "invalid_page"},
		{name: "page_size", path: "/dispatch-events?page_size=0", errorCode: "invalid_page_size"},
		{name: "page_size max", path: "/dispatch-events?page_size=201", errorCode: "invalid_page_size"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeDispatchEventRepository{}
			router := newDispatchEventHandlerTestRouter(repo, time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC))
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d, body=%s, want 400", tt.path, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), `"error":"`+tt.errorCode+`"`) {
				t.Fatalf("body = %s, want error %s", resp.Body.String(), tt.errorCode)
			}
		})
	}
}

func TestDispatchEventHandlerRequiresRequestTenantScope(t *testing.T) {
	repo := &fakeDispatchEventRepository{}
	handler := NewDispatchEventHandler(repo)
	handler.now = func() time.Time { return time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC) }
	router := gin.New()
	router.GET("/dispatch-events", handler.ListDispatchEvents)

	req := httptest.NewRequest(http.MethodGet, "/dispatch-events?tenantId=246", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("GET /dispatch-events without request tenant = %d, body=%s, want 401", resp.Code, resp.Body.String())
	}
}

func TestDispatchEventHandlerReturnsInternalErrorForRepositoryFailures(t *testing.T) {
	repoErr := errors.New("database unavailable")
	cases := []struct {
		name      string
		path      string
		errorCode string
		repo      *fakeDispatchEventRepository
	}{
		{
			name:      "summary",
			path:      "/dispatch-events/summary",
			errorCode: "dispatch_event_summary_failed",
			repo:      &fakeDispatchEventRepository{summaryErr: repoErr},
		},
		{
			name:      "list",
			path:      "/dispatch-events",
			errorCode: "dispatch_event_list_failed",
			repo:      &fakeDispatchEventRepository{listErr: repoErr},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router := newDispatchEventHandlerTestRouter(tt.repo, time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC))
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusInternalServerError {
				t.Fatalf("GET %s = %d, body=%s, want 500", tt.path, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), `"error":"`+tt.errorCode+`"`) {
				t.Fatalf("body = %s, want error %s", resp.Body.String(), tt.errorCode)
			}
		})
	}
}

func newDispatchEventHandlerTestRouter(repo *fakeDispatchEventRepository, now time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewDispatchEventHandler(repo)
	handler.now = func() time.Time { return now }
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Tenant-ID") == "" {
			c.Request.Header.Set("X-Tenant-ID", "246")
		}
		c.Next()
	})
	router.GET("/dispatch-events/summary", handler.GetDispatchEventSummary)
	router.GET("/dispatch-events", handler.ListDispatchEvents)
	return router
}
