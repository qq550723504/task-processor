# Listing Dispatch Event Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a backend API and lightweight ListingKit admin page for viewing `listing_dispatch_event` summary and recent events.

**Architecture:** Add a focused `listingadmin` dispatch-event repository/handler pair, expose it through the existing ListingKit admin route descriptor chain, then add a typed frontend API module and admin page. Keep backend deployable independently from frontend publication.

**Tech Stack:** Go, GORM/Postgres, Gin, existing `internal/listingadmin` admin patterns, Next.js App Router, React Query, Zod, Vitest.

---

## File structure

Backend files:

- Create `internal/listingadmin/dispatch_event_observability.go`
  - Public query, DTO, page, summary, and repository interface types.
- Create `internal/listingadmin/dispatch_event_observability_repository.go`
  - GORM implementation against `listing_dispatch_event`.
- Create `internal/listingadmin/dispatch_event_observability_handler.go`
  - Gin handler for summary and list endpoints.
- Create `internal/listingadmin/dispatch_event_observability_repository_test.go`
  - Repository tests for summary/list/filter/window behavior.
- Create `internal/listingadmin/dispatch_event_observability_handler_test.go`
  - Handler tests for response shape and invalid time.
- Modify `internal/listingkit/api/handler.go`
  - Add repository dependency and handler field.
- Modify `internal/listingkit/api/admin_store_handler.go`
  - Add wrapper methods and availability guard.
- Modify `internal/listingkit/api/admin_dependencies.go` or the existing dependency wiring file that defines `withStoreAdminDependencies`
  - Build the new handler when the repository is provided.
- Modify `internal/listingkit/httpapi/route_handler*.go`
  - Add route handler methods to the admin route interface.
- Modify `internal/listingkit/httpapi/routes_descriptor_admin_store.go`
  - Add the two admin routes.
- Modify `internal/app/httpapi/server_test.go`
  - Extend the stub and route smoke tests.
- Modify the production handler dependency assembly where `AdminHandlerDependencies` is built
  - Pass the existing local import-task repository as the dispatch-event repository if it implements the new interface, or construct the new GORM repository from the existing DB dependency.

Frontend files:

- Create `web/listingkit-ui/src/lib/api/admin-dispatch-events.ts`
  - Zod schemas, response parsers, and API calls.
- Create `web/listingkit-ui/src/lib/api/admin-dispatch-events.test.ts`
  - API schema and request URL tests.
- Create `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.tsx`
  - Admin page component.
- Create `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.test.tsx`
  - Rendering and empty-state tests.
- Create `web/listingkit-ui/src/app/listing-kits/admin/dispatch-events/page.tsx`
  - App Router entrypoint.
- Modify `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
  - Add a `调度事件` admin navigation item if the existing nav list is straightforward.
- Modify `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`
  - Add nav assertion if nav was changed.

Docs:

- Modify `docs/refactoring/listingkit-refactoring-progress-2026-06-24.md`
  - Add a short note that dispatch event observability is implemented but frontend publication may still be deferred.

---

## Task 1: Backend repository contract and tests

**Files:**

- Create: `internal/listingadmin/dispatch_event_observability.go`
- Create: `internal/listingadmin/dispatch_event_observability_repository_test.go`

- [ ] **Step 1: Add the public types**

Create `internal/listingadmin/dispatch_event_observability.go`:

```go
package listingadmin

import (
	"context"
	"time"
)

const DispatchEventDefaultWindow = time.Hour

type DispatchEventQuery struct {
	TenantID    int64
	OwnerUserID string
	Platform    string
	StoreID     *int64
	Action      string
	ReasonCode  string
	From        time.Time
	To          time.Time
	Page        int
	PageSize    int
}

type DispatchEventWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type DispatchEventReasonCount struct {
	ReasonCode string `json:"reasonCode"`
	Action     string `json:"action"`
	Count      int64  `json:"count"`
}

type DispatchEventStoreBlocker struct {
	TenantID          int64  `json:"tenantId"`
	StoreID           int64  `json:"storeId"`
	ReasonCode        string `json:"reasonCode"`
	Count             int64  `json:"count"`
	DailyLimit        int    `json:"dailyLimit"`
	MaxQueued         int    `json:"maxQueued"`
	MaxProcessing     int    `json:"maxProcessing"`
	MaxCompletedToday int    `json:"maxCompletedToday"`
	OwnerNode         string `json:"ownerNode,omitempty"`
}

type DispatchEventSummary struct {
	Window        DispatchEventWindow          `json:"window"`
	Total         int64                        `json:"total"`
	Dispatched    int64                        `json:"dispatched"`
	Skipped       int64                        `json:"skipped"`
	Failed        int64                        `json:"failed"`
	ReasonCounts  []DispatchEventReasonCount  `json:"reasonCounts"`
	StoreBlockers []DispatchEventStoreBlocker `json:"storeBlockers"`
}

type DispatchEventItem struct {
	ID             int64     `json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	TaskID         int64     `json:"taskId"`
	TenantID       int64     `json:"tenantId"`
	StoreID        int64     `json:"storeId"`
	Platform       string    `json:"platform,omitempty"`
	Action         string    `json:"action"`
	ReasonCode     string    `json:"reasonCode,omitempty"`
	Stage          string    `json:"stage,omitempty"`
	Capacity       int       `json:"capacity"`
	Queued         int       `json:"queued"`
	Processing     int       `json:"processing"`
	CompletedToday int       `json:"completedToday"`
	DailyLimit     int       `json:"dailyLimit"`
	OwnerNode      string    `json:"ownerNode,omitempty"`
}

type DispatchEventPage struct {
	Items    []DispatchEventItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type DispatchEventRepository interface {
	GetDispatchEventSummary(ctx context.Context, query DispatchEventQuery) (*DispatchEventSummary, error)
	ListDispatchEvents(ctx context.Context, query DispatchEventQuery) (*DispatchEventPage, error)
}
```

- [ ] **Step 2: Write failing repository tests**

Create `internal/listingadmin/dispatch_event_observability_repository_test.go`:

```go
package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDispatchEventObservabilityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingDispatchEvent{}); err != nil {
		t.Fatalf("migrate dispatch event: %v", err)
	}
	return db
}

func seedDispatchEventRows(t *testing.T, db *gorm.DB, rows ...listingDispatchEvent) {
	t.Helper()
	for _, row := range rows {
		if row.CreatedAt.IsZero() {
			row.CreatedAt = time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed dispatch event: %v", err)
		}
	}
}

func TestDispatchEventSummaryAggregatesActionsReasonsAndStoreBlockers(t *testing.T) {
	db := newDispatchEventObservabilityDB(t)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	seedDispatchEventRows(t, db,
		listingDispatchEvent{TaskID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Action: "skipped", ReasonCode: "no_capacity", Capacity: 8, Queued: 8, DailyLimit: 500, OwnerNode: "node-a", CreatedAt: now.Add(-10 * time.Minute)},
		listingDispatchEvent{TaskID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Action: "skipped", ReasonCode: "no_capacity", Capacity: 8, Queued: 7, DailyLimit: 500, OwnerNode: "node-a", CreatedAt: now.Add(-9 * time.Minute)},
		listingDispatchEvent{TaskID: 3, TenantID: 10, StoreID: 101, Platform: "shein", Action: "skipped", ReasonCode: "store_paused", Capacity: 2, Queued: 0, OwnerNode: "node-b", CreatedAt: now.Add(-8 * time.Minute)},
		listingDispatchEvent{TaskID: 4, TenantID: 10, StoreID: 102, Platform: "shein", Action: "dispatched", Capacity: 2, Queued: 1, DailyLimit: 800, OwnerNode: "node-c", CreatedAt: now.Add(-7 * time.Minute)},
	)

	repo := NewGormDispatchEventRepository(db)
	summary, err := repo.GetDispatchEventSummary(context.Background(), DispatchEventQuery{
		TenantID: 10,
		From:     now.Add(-time.Hour),
		To:       now,
	})
	if err != nil {
		t.Fatalf("GetDispatchEventSummary: %v", err)
	}
	if summary.Total != 4 || summary.Dispatched != 1 || summary.Skipped != 3 || summary.Failed != 0 {
		t.Fatalf("summary counts = total:%d dispatched:%d skipped:%d failed:%d", summary.Total, summary.Dispatched, summary.Skipped, summary.Failed)
	}
	if len(summary.ReasonCounts) != 3 {
		t.Fatalf("reason counts len = %d, want 3", len(summary.ReasonCounts))
	}
	if summary.ReasonCounts[0].ReasonCode != "no_capacity" || summary.ReasonCounts[0].Count != 2 {
		t.Fatalf("top reason = %+v, want no_capacity count 2", summary.ReasonCounts[0])
	}
	if len(summary.StoreBlockers) == 0 || summary.StoreBlockers[0].StoreID != 100 || summary.StoreBlockers[0].Count != 2 || summary.StoreBlockers[0].MaxQueued != 8 {
		t.Fatalf("top blocker = %+v", summary.StoreBlockers)
	}
}

func TestListDispatchEventsFiltersAndPagesRecentRows(t *testing.T) {
	db := newDispatchEventObservabilityDB(t)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	seedDispatchEventRows(t, db,
		listingDispatchEvent{TaskID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Action: "skipped", ReasonCode: "no_capacity", CreatedAt: now.Add(-10 * time.Minute)},
		listingDispatchEvent{TaskID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Action: "dispatched", CreatedAt: now.Add(-9 * time.Minute)},
		listingDispatchEvent{TaskID: 3, TenantID: 11, StoreID: 100, Platform: "shein", Action: "skipped", ReasonCode: "no_capacity", CreatedAt: now.Add(-8 * time.Minute)},
	)

	repo := NewGormDispatchEventRepository(db)
	page, err := repo.ListDispatchEvents(context.Background(), DispatchEventQuery{
		TenantID:   10,
		StoreID:    int64Ptr(100),
		Action:     "skipped",
		ReasonCode: "no_capacity",
		From:       now.Add(-time.Hour),
		To:         now,
		Page:       1,
		PageSize:   20,
	})
	if err != nil {
		t.Fatalf("ListDispatchEvents: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page total/items = %d/%d, want 1/1", page.Total, len(page.Items))
	}
	if page.Items[0].TaskID != 1 || page.Items[0].ReasonCode != "no_capacity" {
		t.Fatalf("item = %+v", page.Items[0])
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```powershell
go test ./internal/listingadmin -run "TestDispatchEvent" -count=1
```

Expected: fail because `NewGormDispatchEventRepository` is undefined.

---

## Task 2: Backend repository implementation

**Files:**

- Create: `internal/listingadmin/dispatch_event_observability_repository.go`
- Modify: `internal/listingadmin/dispatch_event_observability_repository_test.go`

- [ ] **Step 1: Add the repository implementation**

Create `internal/listingadmin/dispatch_event_observability_repository.go`:

```go
package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const dispatchEventMaxPageSize = 200

type GormDispatchEventRepository struct {
	db *gorm.DB
}

func NewGormDispatchEventRepository(db *gorm.DB) *GormDispatchEventRepository {
	return &GormDispatchEventRepository{db: db}
}

func (r *GormDispatchEventRepository) GetDispatchEventSummary(ctx context.Context, query DispatchEventQuery) (*DispatchEventSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("dispatch event repository database is not configured")
	}
	query = normalizeDispatchEventQuery(query)
	base := applyDispatchEventQuery(r.db.WithContext(ctx).Table("listing_dispatch_event"), query)

	var actionRows []struct {
		Action string
		Count  int64
	}
	if err := base.Select("action, count(*) as count").Group("action").Scan(&actionRows).Error; err != nil {
		return nil, err
	}

	var total, dispatched, skipped, failed int64
	for _, row := range actionRows {
		total += row.Count
		switch strings.TrimSpace(row.Action) {
		case "dispatched":
			dispatched += row.Count
		case "skipped":
			skipped += row.Count
		case "failed":
			failed += row.Count
		}
	}

	var reasonRows []DispatchEventReasonCount
	if err := base.
		Select("COALESCE(NULLIF(reason_code, ''), '<dispatched>') as reason_code, action, count(*) as count").
		Group("COALESCE(NULLIF(reason_code, ''), '<dispatched>'), action").
		Order("count desc, reason_code asc").
		Scan(&reasonRows).Error; err != nil {
		return nil, err
	}

	var blockerRows []DispatchEventStoreBlocker
	if err := base.
		Where("action = ?", "skipped").
		Select(`tenant_id, store_id, reason_code, count(*) as count,
			max(daily_limit) as daily_limit,
			max(queued) as max_queued,
			max(processing) as max_processing,
			max(completed_today) as max_completed_today,
			max(owner_node) as owner_node`).
		Group("tenant_id, store_id, reason_code").
		Order("count desc, tenant_id asc, store_id asc, reason_code asc").
		Limit(20).
		Scan(&blockerRows).Error; err != nil {
		return nil, err
	}

	return &DispatchEventSummary{
		Window:        DispatchEventWindow{From: query.From, To: query.To},
		Total:         total,
		Dispatched:    dispatched,
		Skipped:       skipped,
		Failed:        failed,
		ReasonCounts:  reasonRows,
		StoreBlockers: blockerRows,
	}, nil
}

func (r *GormDispatchEventRepository) ListDispatchEvents(ctx context.Context, query DispatchEventQuery) (*DispatchEventPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("dispatch event repository database is not configured")
	}
	query = normalizeDispatchEventQuery(query)
	base := applyDispatchEventQuery(r.db.WithContext(ctx).Table("listing_dispatch_event"), query)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []listingDispatchEvent
	offset := (query.Page - 1) * query.PageSize
	if err := base.Order("created_at desc, id desc").Offset(offset).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]DispatchEventItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dispatchEventItemFromRow(row))
	}
	return &DispatchEventPage{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func normalizeDispatchEventQuery(query DispatchEventQuery) DispatchEventQuery {
	now := time.Now()
	switch {
	case query.From.IsZero() && query.To.IsZero():
		query.To = now
		query.From = now.Add(-DispatchEventDefaultWindow)
	case query.From.IsZero():
		query.From = query.To.Add(-DispatchEventDefaultWindow)
	case query.To.IsZero():
		query.To = now
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.PageSize > dispatchEventMaxPageSize {
		query.PageSize = dispatchEventMaxPageSize
	}
	query.Platform = strings.TrimSpace(query.Platform)
	query.Action = strings.TrimSpace(query.Action)
	query.ReasonCode = strings.TrimSpace(query.ReasonCode)
	query.OwnerUserID = strings.TrimSpace(query.OwnerUserID)
	return query
}

func applyDispatchEventQuery(db *gorm.DB, query DispatchEventQuery) *gorm.DB {
	db = db.Where("created_at >= ? AND created_at <= ?", query.From, query.To)
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.ReasonCode != "" {
		db = db.Where("reason_code = ?", query.ReasonCode)
	}
	return db
}

func dispatchEventItemFromRow(row listingDispatchEvent) DispatchEventItem {
	return DispatchEventItem{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		TaskID:         row.TaskID,
		TenantID:       row.TenantID,
		StoreID:        row.StoreID,
		Platform:       row.Platform,
		Action:         row.Action,
		ReasonCode:     row.ReasonCode,
		Stage:          row.Stage,
		Capacity:       row.Capacity,
		Queued:         row.Queued,
		Processing:     row.Processing,
		CompletedToday: row.CompletedToday,
		DailyLimit:     row.DailyLimit,
		OwnerNode:      row.OwnerNode,
	}
}
```

- [ ] **Step 2: Fix test helper if needed**

If `int64Ptr` is not available in this package test, add this helper to `internal/listingadmin/dispatch_event_observability_repository_test.go`:

```go
func int64Ptr(value int64) *int64 {
	return &value
}
```

- [ ] **Step 3: Run repository tests**

Run:

```powershell
go test ./internal/listingadmin -run "TestDispatchEvent" -count=1
```

Expected: pass.

- [ ] **Step 4: Commit**

```powershell
git add internal/listingadmin/dispatch_event_observability.go internal/listingadmin/dispatch_event_observability_repository.go internal/listingadmin/dispatch_event_observability_repository_test.go
git commit -m "Add listing dispatch event observability repository"
```

---

## Task 3: Backend handler tests and implementation

**Files:**

- Create: `internal/listingadmin/dispatch_event_observability_handler.go`
- Create: `internal/listingadmin/dispatch_event_observability_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/listingadmin/dispatch_event_observability_handler_test.go`:

```go
package listingadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeDispatchEventRepository struct {
	summaryQuery DispatchEventQuery
	listQuery    DispatchEventQuery
}

func (f *fakeDispatchEventRepository) GetDispatchEventSummary(_ context.Context, query DispatchEventQuery) (*DispatchEventSummary, error) {
	f.summaryQuery = query
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
	return &DispatchEventPage{
		Items: []DispatchEventItem{{ID: 1, TaskID: 101, TenantID: query.TenantID, StoreID: 976, Action: "skipped", ReasonCode: "no_capacity"}},
		Total: 1,
		Page:  query.Page,
		PageSize: query.PageSize,
	}, nil
}

func TestDispatchEventHandlerSummaryParsesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeDispatchEventRepository{}
	handler := NewDispatchEventHandler(repo)
	router := gin.New()
	router.GET("/dispatch-events/summary", func(c *gin.Context) {
		c.Set("tenant_id", int64(246))
		handler.GetDispatchEventSummary(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/dispatch-events/summary?storeId=976&action=skipped&reasonCode=no_capacity&from=2026-06-24T14:00:00Z&to=2026-06-24T15:00:00Z", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if repo.summaryQuery.TenantID != 246 || repo.summaryQuery.StoreID == nil || *repo.summaryQuery.StoreID != 976 || repo.summaryQuery.Action != "skipped" || repo.summaryQuery.ReasonCode != "no_capacity" {
		t.Fatalf("query = %+v", repo.summaryQuery)
	}
	if repo.summaryQuery.From.IsZero() || repo.summaryQuery.To.IsZero() {
		t.Fatalf("time window was not parsed: %+v", repo.summaryQuery)
	}
}

func TestDispatchEventHandlerListParsesPaging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeDispatchEventRepository{}
	handler := NewDispatchEventHandler(repo)
	router := gin.New()
	router.GET("/dispatch-events", func(c *gin.Context) {
		c.Set("tenant_id", int64(246))
		handler.ListDispatchEvents(c)
	})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/dispatch-events?page=2&page_size=25", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if repo.listQuery.Page != 2 || repo.listQuery.PageSize != 25 {
		t.Fatalf("paging = %d/%d, want 2/25", repo.listQuery.Page, repo.listQuery.PageSize)
	}
}

func TestDispatchEventHandlerInvalidTimeReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDispatchEventHandler(&fakeDispatchEventRepository{})
	router := gin.New()
	router.GET("/dispatch-events/summary", handler.GetDispatchEventSummary)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/dispatch-events/summary?from=not-a-time", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
}

var _ = time.RFC3339
```

- [ ] **Step 2: Run handler tests to verify failure**

Run:

```powershell
go test ./internal/listingadmin -run "TestDispatchEventHandler" -count=1
```

Expected: fail because `NewDispatchEventHandler` is undefined.

- [ ] **Step 3: Implement handler**

Create `internal/listingadmin/dispatch_event_observability_handler.go`:

```go
package listingadmin

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type DispatchEventHandler struct {
	repo DispatchEventRepository
}

func NewDispatchEventHandler(repo DispatchEventRepository) *DispatchEventHandler {
	return &DispatchEventHandler{repo: repo}
}

func (h *DispatchEventHandler) GetDispatchEventSummary(c *gin.Context) {
	query, ok := dispatchEventQueryFromRequest(c, false)
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
	query, ok := dispatchEventQueryFromRequest(c, true)
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

func dispatchEventQueryFromRequest(c *gin.Context, includePaging bool) (DispatchEventQuery, bool) {
	scope := requestListScope(c)
	query := DispatchEventQuery{
		TenantID:    scope.TenantID,
		OwnerUserID: scope.OwnerUserID,
		Platform:    strings.TrimSpace(c.Query("platform")),
		Action:      strings.TrimSpace(c.Query("action")),
		ReasonCode:  strings.TrimSpace(c.Query("reasonCode")),
		Page:        scope.Page,
		PageSize:    scope.PageSize,
	}
	if !includePaging {
		query.Page = 1
		query.PageSize = 50
	}
	storeID, ok := queryInt64PtrStrict(c, "storeId", "invalid_store_id")
	if !ok {
		return DispatchEventQuery{}, false
	}
	query.StoreID = storeID
	from, ok := queryOptionalRFC3339(c, "from")
	if !ok {
		return DispatchEventQuery{}, false
	}
	to, ok := queryOptionalRFC3339(c, "to")
	if !ok {
		return DispatchEventQuery{}, false
	}
	query.From = from
	query.To = to
	return query, true
}

func queryOptionalRFC3339(c *gin.Context, key string) (time.Time, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		writeValidationError(c, "invalid_dispatch_event_time_range", err)
		return time.Time{}, false
	}
	return parsed, true
}
```

- [ ] **Step 4: Run handler tests**

Run:

```powershell
go test ./internal/listingadmin -run "TestDispatchEventHandler" -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingadmin/dispatch_event_observability_handler.go internal/listingadmin/dispatch_event_observability_handler_test.go
git commit -m "Add listing dispatch event admin handler"
```

---

## Task 4: Wire ListingKit API routes

**Files:**

- Modify: `internal/listingkit/api/handler.go`
- Modify: `internal/listingkit/api/admin_store_handler.go`
- Modify: existing dependency wiring file containing `withStoreAdminDependencies`
- Modify: `internal/listingkit/httpapi/routes_descriptor_admin_store.go`
- Modify: `internal/listingkit/httpapi/*route*handler*.go` where `AdminRouteHandler` is defined
- Modify: `internal/app/httpapi/server_test.go`

- [ ] **Step 1: Add route smoke tests**

In `internal/app/httpapi/server_test.go`, extend `stubListingKitHandler`:

```go
listAdminDispatchEventSummaryCalled bool
listAdminDispatchEventsCalled       bool
```

Add methods:

```go
func (s *stubListingKitHandler) GetAdminDispatchEventSummary(c *gin.Context) {
	s.listAdminDispatchEventSummaryCalled = true
	c.JSON(http.StatusOK, gin.H{"total": 0})
}

func (s *stubListingKitHandler) ListAdminDispatchEvents(c *gin.Context) {
	s.listAdminDispatchEventsCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0, "page": 1, "page_size": 50})
}
```

Add smoke assertions near the existing admin route tests:

```go
handler.listAdminDispatchEventSummaryCalled = false
req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/dispatch-events/summary", nil)
resp = httptest.NewRecorder()
router.ServeHTTP(resp, req)
if resp.Code != http.StatusOK {
	t.Fatalf("GET /api/v1/listing-kits/admin/dispatch-events/summary = %d, want 200", resp.Code)
}
if !handler.listAdminDispatchEventSummaryCalled {
	t.Fatal("listing kit GetAdminDispatchEventSummary handler was not called")
}

handler.listAdminDispatchEventsCalled = false
req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/dispatch-events", nil)
resp = httptest.NewRecorder()
router.ServeHTTP(resp, req)
if resp.Code != http.StatusOK {
	t.Fatalf("GET /api/v1/listing-kits/admin/dispatch-events = %d, want 200", resp.Code)
}
if !handler.listAdminDispatchEventsCalled {
	t.Fatal("listing kit ListAdminDispatchEvents handler was not called")
}
```

- [ ] **Step 2: Run route tests to verify failure**

Run:

```powershell
go test ./internal/app/httpapi -run "Test.*ListingKit.*Admin" -count=1
```

Expected: fail because route handler interface and route descriptors do not include the new methods yet.

- [ ] **Step 3: Wire API handler dependency**

In `internal/listingkit/api/handler.go`, add to `storeAdminHandlers`:

```go
dispatchEventHandler *listingadmin.DispatchEventHandler
```

Add to `AdminHandlerDependencies`:

```go
DispatchEventRepository listingadmin.DispatchEventRepository
```

In the existing `withStoreAdminDependencies` function, add:

```go
withAdminDependency(deps.DispatchEventRepository, func(repo listingadmin.DispatchEventRepository, admin *adminHandlers) {
	admin.dispatchEventHandler = listingadmin.NewDispatchEventHandler(repo)
})
```

- [ ] **Step 4: Add API wrapper methods**

In `internal/listingkit/api/admin_store_handler.go`, add:

```go
func (h *handler) GetAdminDispatchEventSummary(c *gin.Context) {
	if !h.requireDispatchEventHandler(c) {
		return
	}
	h.dispatchEventHandler.GetDispatchEventSummary(c)
}

func (h *handler) ListAdminDispatchEvents(c *gin.Context) {
	if !h.requireDispatchEventHandler(c) {
		return
	}
	h.dispatchEventHandler.ListDispatchEvents(c)
}

func (h *handler) requireDispatchEventHandler(c *gin.Context) bool {
	if h.dispatchEventHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "dispatch_event_repository_unavailable",
		"message": "ListingKit dispatch event repository is not configured",
	})
	return false
}
```

- [ ] **Step 5: Add HTTP route interface and descriptors**

Where `AdminRouteHandler` is defined, add:

```go
GetAdminDispatchEventSummary(c *gin.Context)
ListAdminDispatchEvents(c *gin.Context)
```

In `internal/listingkit/httpapi/routes_descriptor_admin_store.go`, add descriptors before import tasks:

```go
httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/dispatch-events/summary", Module: "listing-kit-admin", Handler: handler.GetAdminDispatchEventSummary},
httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/admin/dispatch-events", Module: "listing-kit-admin", Handler: handler.ListAdminDispatchEvents},
```

- [ ] **Step 6: Pass repository through runtime dependency assembly**

Find where `api.AdminHandlerDependencies` is constructed. Add:

```go
DispatchEventRepository: listingadmin.NewGormDispatchEventRepository(db),
```

If only an existing `ImportTaskRepository` is available there and it uses the same DB, prefer adding a dedicated `NewGormDispatchEventRepository(db)` to avoid bloating import task responsibilities.

- [ ] **Step 7: Run backend route and listingadmin tests**

Run:

```powershell
go test ./internal/listingadmin ./internal/listingkit/... ./internal/app/httpapi -run "TestDispatchEvent|Test.*ListingKit.*Admin" -count=1
```

Expected: pass.

- [ ] **Step 8: Commit**

```powershell
git add internal/listingkit internal/app/httpapi internal/listingadmin
git commit -m "Expose listing dispatch event admin APIs"
```

---

## Task 5: Frontend API module

**Files:**

- Create: `web/listingkit-ui/src/lib/api/admin-dispatch-events.ts`
- Create: `web/listingkit-ui/src/lib/api/admin-dispatch-events.test.ts`

- [ ] **Step 1: Write API tests**

Create `web/listingkit-ui/src/lib/api/admin-dispatch-events.test.ts`:

```typescript
import {
  parseDispatchEventPageResponse,
  parseDispatchEventSummaryResponse,
} from "@/lib/api/admin-dispatch-events";

describe("admin dispatch event API schemas", () => {
  it("parses summary responses", () => {
    const parsed = parseDispatchEventSummaryResponse({
      window: {
        from: "2026-06-24T14:00:00Z",
        to: "2026-06-24T15:00:00Z",
      },
      total: 3,
      dispatched: 1,
      skipped: 2,
      failed: 0,
      reasonCounts: [{ reasonCode: "no_capacity", action: "skipped", count: 2 }],
      storeBlockers: [{ tenantId: 246, storeId: 1041, reasonCode: "no_capacity", count: 2, dailyLimit: 500, maxQueued: 8, maxProcessing: 0, maxCompletedToday: 0, ownerNode: "node-a" }],
    });

    expect(parsed.total).toBe(3);
    expect(parsed.reasonCounts[0].reasonCode).toBe("no_capacity");
  });

  it("parses paged event responses", () => {
    const parsed = parseDispatchEventPageResponse({
      items: [
        {
          id: 1,
          createdAt: "2026-06-24T14:54:09Z",
          taskId: 8417710,
          tenantId: 246,
          storeId: 1041,
          platform: "shein",
          action: "skipped",
          reasonCode: "no_capacity",
          stage: "dispatch",
          capacity: 8,
          queued: 8,
          processing: 0,
          completedToday: 0,
          dailyLimit: 500,
          ownerNode: "node-a",
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
    });

    expect(parsed.items[0].taskId).toBe(8417710);
  });
});
```

- [ ] **Step 2: Run API tests to verify failure**

Run:

```powershell
cd web/listingkit-ui
npm test -- admin-dispatch-events.test.ts
```

Expected: fail because module does not exist.

- [ ] **Step 3: Implement API module**

Create `web/listingkit-ui/src/lib/api/admin-dispatch-events.ts`:

```typescript
import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

const dispatchEventWindowSchema = z.object({
  from: z.string(),
  to: z.string(),
});

export const dispatchEventReasonCountSchema = z
  .object({
    reasonCode: z.string(),
    action: z.string(),
    count: z.number(),
  })
  .passthrough();

export const dispatchEventStoreBlockerSchema = z
  .object({
    tenantId: z.number(),
    storeId: z.number(),
    reasonCode: z.string(),
    count: z.number(),
    dailyLimit: z.number(),
    maxQueued: z.number(),
    maxProcessing: z.number(),
    maxCompletedToday: z.number(),
    ownerNode: z.string().optional(),
  })
  .passthrough();

export const dispatchEventSummarySchema = z
  .object({
    window: dispatchEventWindowSchema,
    total: z.number(),
    dispatched: z.number(),
    skipped: z.number(),
    failed: z.number(),
    reasonCounts: z.array(dispatchEventReasonCountSchema),
    storeBlockers: z.array(dispatchEventStoreBlockerSchema),
  })
  .passthrough();

export const dispatchEventItemSchema = z
  .object({
    id: z.number(),
    createdAt: z.string(),
    taskId: z.number(),
    tenantId: z.number(),
    storeId: z.number(),
    platform: z.string().optional(),
    action: z.string(),
    reasonCode: z.string().optional(),
    stage: z.string().optional(),
    capacity: z.number(),
    queued: z.number(),
    processing: z.number(),
    completedToday: z.number(),
    dailyLimit: z.number(),
    ownerNode: z.string().optional(),
  })
  .passthrough();

export const dispatchEventPageSchema = z
  .object({
    items: z.array(dispatchEventItemSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type DispatchEventSummary = z.infer<typeof dispatchEventSummarySchema>;
export type DispatchEventPage = z.infer<typeof dispatchEventPageSchema>;
export type DispatchEventItem = z.infer<typeof dispatchEventItemSchema>;

export type DispatchEventQuery = QueueQuery & {
  platform?: string;
  storeId?: number;
  action?: string;
  reasonCode?: string;
  from?: string;
  to?: string;
  page?: number;
  page_size?: number;
};

export function parseDispatchEventSummaryResponse(
  payload: unknown,
): DispatchEventSummary {
  return parseApiResponseShape(
    payload,
    dispatchEventSummarySchema,
    "ListingKit API returned an unexpected dispatch event summary response",
  );
}

export function parseDispatchEventPageResponse(payload: unknown): DispatchEventPage {
  return parseApiResponseShape(
    payload,
    dispatchEventPageSchema,
    "ListingKit API returned an unexpected dispatch event page response",
  );
}

export async function getListingDispatchEventSummary(
  query: DispatchEventQuery = {},
): Promise<DispatchEventSummary> {
  const payload = await apiRequest<unknown>("/admin/dispatch-events/summary", {
    query,
  });
  return parseDispatchEventSummaryResponse(payload);
}

export async function getListingDispatchEvents(
  query: DispatchEventQuery = {},
): Promise<DispatchEventPage> {
  const payload = await apiRequest<unknown>("/admin/dispatch-events", { query });
  return parseDispatchEventPageResponse(payload);
}
```

- [ ] **Step 4: Run API tests**

Run:

```powershell
cd web/listingkit-ui
npm test -- admin-dispatch-events.test.ts
```

Expected: pass.

- [ ] **Step 5: Commit**

```powershell
git add web/listingkit-ui/src/lib/api/admin-dispatch-events.ts web/listingkit-ui/src/lib/api/admin-dispatch-events.test.ts
git commit -m "Add listing dispatch event frontend API"
```

---

## Task 6: Frontend admin page

**Files:**

- Create: `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.test.tsx`
- Create: `web/listingkit-ui/src/app/listing-kits/admin/dispatch-events/page.tsx`

- [ ] **Step 1: Write page rendering test**

Create `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.test.tsx`:

```typescript
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DispatchEventAdminPage } from "@/components/listingkit/admin/dispatch-event-admin-page";
import * as api from "@/lib/api/admin-dispatch-events";

describe("DispatchEventAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders summary, blockers, and recent events", async () => {
    vi.spyOn(api, "getListingDispatchEventSummary").mockResolvedValue({
      window: { from: "2026-06-24T14:00:00Z", to: "2026-06-24T15:00:00Z" },
      total: 3,
      dispatched: 1,
      skipped: 2,
      failed: 0,
      reasonCounts: [{ reasonCode: "no_capacity", action: "skipped", count: 2 }],
      storeBlockers: [{ tenantId: 246, storeId: 1041, reasonCode: "no_capacity", count: 2, dailyLimit: 500, maxQueued: 8, maxProcessing: 0, maxCompletedToday: 0, ownerNode: "node-a" }],
    });
    vi.spyOn(api, "getListingDispatchEvents").mockResolvedValue({
      items: [{ id: 1, createdAt: "2026-06-24T14:54:09Z", taskId: 8417710, tenantId: 246, storeId: 1041, platform: "shein", action: "skipped", reasonCode: "no_capacity", stage: "dispatch", capacity: 8, queued: 8, processing: 0, completedToday: 0, dailyLimit: 500, ownerNode: "node-a" }],
      total: 1,
      page: 1,
      page_size: 50,
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <DispatchEventAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "调度事件" })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("no_capacity")).toBeInTheDocument());
    expect(screen.getByText("8417710")).toBeInTheDocument();
    expect(screen.getByText("node-a")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run page test to verify failure**

Run:

```powershell
cd web/listingkit-ui
npm test -- dispatch-event-admin-page.test.tsx
```

Expected: fail because component does not exist.

- [ ] **Step 3: Implement page component**

Create `web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.tsx` using the existing admin page style:

```typescript
"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  getListingDispatchEventSummary,
  getListingDispatchEvents,
  type DispatchEventQuery,
} from "@/lib/api/admin-dispatch-events";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw, Search } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

export function DispatchEventAdminPage() {
  const [platform, setPlatform] = useState("shein");
  const [storeId, setStoreId] = useState("");
  const [action, setAction] = useState("");
  const [reasonCode, setReasonCode] = useState("");

  const query = useMemo<DispatchEventQuery>(
    () => ({
      platform: platform || undefined,
      storeId: storeId ? Number(storeId) : undefined,
      action: action || undefined,
      reasonCode: reasonCode || undefined,
      page: 1,
      page_size: 50,
    }),
    [action, platform, reasonCode, storeId],
  );

  const summaryQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-event-summary", query],
    queryFn: () => getListingDispatchEventSummary(query),
  });
  const eventsQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-events", query],
    queryFn: () => getListingDispatchEvents(query),
  });

  const loading = summaryQuery.isLoading || summaryQuery.isFetching || eventsQuery.isLoading || eventsQuery.isFetching;
  const visibleError =
    summaryQuery.error instanceof Error
      ? formatSubscriptionApiError(summaryQuery.error)
      : eventsQuery.error instanceof Error
        ? formatSubscriptionApiError(eventsQuery.error)
        : "";

  async function handleRefresh(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    await Promise.all([summaryQuery.refetch(), eventsQuery.refetch()]);
  }

  const summary = summaryQuery.data;
  const page = eventsQuery.data;

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">调度事件</h1>
            <p className="mt-1 text-sm text-zinc-500">
              查看 Go Listing Control Plane 最近的 dispatch / skipped 决策。
            </p>
          </div>
          <form className="flex flex-col gap-2 sm:flex-row sm:flex-wrap" onSubmit={handleRefresh}>
            <FilterInput label="平台" value={platform} onChange={setPlatform} />
            <FilterInput label="店铺 ID" value={storeId} onChange={setStoreId} type="number" />
            <FilterSelect label="动作" value={action} onChange={setAction} options={["", "dispatched", "skipped", "failed"]} />
            <FilterInput label="原因" value={reasonCode} onChange={setReasonCode} placeholder="no_capacity" />
            <Button type="submit" className="w-full sm:mt-5 sm:w-auto" variant="secondary">
              {loading ? <RefreshCw className="size-4 animate-spin" /> : <Search className="size-4" />}
              查询
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      <section className="grid gap-3 md:grid-cols-4">
        <MetricCard label="总事件" value={summary?.total ?? 0} />
        <MetricCard label="已派发" value={summary?.dispatched ?? 0} />
        <MetricCard label="已跳过" value={summary?.skipped ?? 0} />
        <MetricCard label="失败" value={summary?.failed ?? 0} />
      </section>

      <section className="grid gap-4 xl:grid-cols-2">
        <Panel title="原因分布">
          {(summary?.reasonCounts ?? []).length === 0 ? (
            <EmptyText>暂无原因分布</EmptyText>
          ) : (
            <div className="space-y-2">
              {summary?.reasonCounts.map((item) => (
                <div key={`${item.action}:${item.reasonCode}`} className="flex items-center justify-between rounded-md border border-zinc-100 px-3 py-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary">{item.reasonCode}</Badge>
                    <span className="text-xs text-zinc-500">{item.action}</span>
                  </div>
                  <span className="font-semibold text-zinc-900">{item.count}</span>
                </div>
              ))}
            </div>
          )}
        </Panel>
        <Panel title="店铺阻塞 Top">
          {(summary?.storeBlockers ?? []).length === 0 ? (
            <EmptyText>暂无阻塞店铺</EmptyText>
          ) : (
            <div className="space-y-2">
              {summary?.storeBlockers.map((item) => (
                <div key={`${item.tenantId}:${item.storeId}:${item.reasonCode}`} className="rounded-md border border-zinc-100 px-3 py-2">
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-zinc-900">#{item.storeId}</span>
                    <Badge variant="neutral">{item.count}</Badge>
                  </div>
                  <div className="mt-1 text-xs text-zinc-500">
                    {item.reasonCode} · daily {item.dailyLimit} · queued {item.maxQueued} · {item.ownerNode || "-"}
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <Table className="min-w-[64rem] divide-y divide-zinc-200 text-sm">
            <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <TableRow>
                <TableHead className="px-4 py-3">时间</TableHead>
                <TableHead className="px-4 py-3">任务</TableHead>
                <TableHead className="px-4 py-3">店铺</TableHead>
                <TableHead className="px-4 py-3">动作</TableHead>
                <TableHead className="px-4 py-3">原因</TableHead>
                <TableHead className="px-4 py-3">容量</TableHead>
                <TableHead className="px-4 py-3">队列</TableHead>
                <TableHead className="px-4 py-3">日限</TableHead>
                <TableHead className="px-4 py-3">Owner</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-zinc-100">
              {(page?.items ?? []).length === 0 ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={9}>
                    暂无调度事件
                  </TableCell>
                </TableRow>
              ) : (
                page?.items.map((item) => (
                  <TableRow key={item.id} className="align-top">
                    <TableCell className="px-4 py-3 text-zinc-600">{formatTime(item.createdAt)}</TableCell>
                    <TableCell className="px-4 py-3 font-mono text-zinc-800">{item.taskId}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.tenantId}/{item.storeId}</TableCell>
                    <TableCell className="px-4 py-3"><Badge variant={item.action === "dispatched" ? "secondary" : "neutral"}>{item.action}</Badge></TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.reasonCode || "-"}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.capacity}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.queued}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.dailyLimit}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.ownerNode || "-"}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </section>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-2 text-2xl font-semibold text-zinc-950">{value}</div>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-base font-semibold text-zinc-950">{title}</h2>
      {children}
    </div>
  );
}

function EmptyText({ children }: { children: React.ReactNode }) {
  return <div className="py-6 text-sm text-zinc-500">{children}</div>;
}

function FilterInput({ label, value, onChange, type = "text", placeholder }: { label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string }) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900" />
    </Label>
  );
}

function FilterSelect({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select value={value} onChange={(event) => onChange(event.target.value)} className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900">
        {options.map((option) => (
          <option key={option} value={option}>{option || "全部"}</option>
        ))}
      </Select>
    </Label>
  );
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}
```

- [ ] **Step 4: Add App Router page**

Create `web/listingkit-ui/src/app/listing-kits/admin/dispatch-events/page.tsx`:

```typescript
import { DispatchEventAdminPage } from "@/components/listingkit/admin/dispatch-event-admin-page";

export default function ListingKitDispatchEventsPage() {
  return <DispatchEventAdminPage />;
}
```

- [ ] **Step 5: Run page test**

Run:

```powershell
cd web/listingkit-ui
npm test -- dispatch-event-admin-page.test.tsx
```

Expected: pass.

- [ ] **Step 6: Commit**

```powershell
git add web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.tsx web/listingkit-ui/src/components/listingkit/admin/dispatch-event-admin-page.test.tsx web/listingkit-ui/src/app/listing-kits/admin/dispatch-events/page.tsx
git commit -m "Add listing dispatch event admin page"
```

---

## Task 7: Navigation and final verification

**Files:**

- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`
- Modify: `docs/refactoring/listingkit-refactoring-progress-2026-06-24.md`

- [ ] **Step 1: Add navigation test**

In `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`, add an assertion alongside the existing admin link assertions:

```typescript
expect(screen.getByRole("link", { name: /调度事件/ })).toHaveAttribute(
  "href",
  "/listing-kits/admin/dispatch-events",
);
```

- [ ] **Step 2: Run shell test to verify failure**

Run:

```powershell
cd web/listingkit-ui
npm test -- listingkit-app-shell.test.tsx
```

Expected: fail because nav link is absent.

- [ ] **Step 3: Add navigation item**

In `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`, add the item near store statistics/import tasks:

```typescript
{
  label: "调度事件",
  href: "/listing-kits/admin/dispatch-events",
}
```

- [ ] **Step 4: Add progress note**

Append to `docs/refactoring/listingkit-refactoring-progress-2026-06-24.md`:

```markdown

### Dispatch event observability implementation

- Added backend admin APIs for `listing_dispatch_event` summary and recent-event listing.
- Added ListingKit UI API schemas and a routable admin page at `/listing-kits/admin/dispatch-events`.
- Frontend publication remains controlled by the separate ListingKit UI deployment window.
```

- [ ] **Step 5: Run focused frontend tests**

Run:

```powershell
cd web/listingkit-ui
npm test -- admin-dispatch-events.test.ts dispatch-event-admin-page.test.tsx listingkit-app-shell.test.tsx
```

Expected: pass.

- [ ] **Step 6: Run focused backend tests**

Run:

```powershell
go test ./internal/listingadmin ./internal/listingkit/... ./internal/app/httpapi -run "TestDispatchEvent|Test.*ListingKit.*Admin" -count=1
```

Expected: pass.

- [ ] **Step 7: Run type checks**

Run:

```powershell
gopls check internal\listingadmin\dispatch_event_observability.go internal\listingadmin\dispatch_event_observability_repository.go internal\listingadmin\dispatch_event_observability_handler.go internal\listingkit\api\handler.go internal\listingkit\api\admin_store_handler.go internal\listingkit\httpapi\routes_descriptor_admin_store.go
```

Expected: no diagnostics.

- [ ] **Step 8: Commit**

```powershell
git add web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx docs/refactoring/listingkit-refactoring-progress-2026-06-24.md
git commit -m "Wire listing dispatch event observability navigation"
```

---

## Task 8: Backend deploy decision

**Files:**

- No code files unless documentation needs a final note.

- [ ] **Step 1: Confirm backend-only deployment scope**

Backend deployment is safe independently because frontend publication is deferred. The backend API can be used by scripts and future UI.

- [ ] **Step 2: If deploying backend, build and deploy the relevant API artifact**

Use the existing deployment process for the ListingKit API service, not the control-plane image, unless the API is bundled with the same application artifact in this environment.

Before running any deployment command, identify the existing script or Kubernetes manifest for the ListingKit API service.

- [ ] **Step 3: If not deploying backend now, document deferred deployment**

Append to `docs/refactoring/listingkit-refactoring-progress-2026-06-24.md`:

```markdown

Dispatch event observability is implemented in code. Backend API deployment and frontend publication are deferred to the next ListingKit deployment window.
```

- [ ] **Step 4: Final commit if docs changed**

```powershell
git add docs/refactoring/listingkit-refactoring-progress-2026-06-24.md
git commit -m "Document dispatch event observability deployment status"
```

---

## Self-review checklist

- Spec coverage:
  - Backend summary endpoint: Task 1 through Task 4.
  - Backend list endpoint: Task 1 through Task 4.
  - Frontend API module: Task 5.
  - Frontend admin page: Task 6.
  - Navigation: Task 7.
  - Error handling: Task 3.
  - Tests: Tasks 1, 3, 4, 5, 6, and 7.
  - Deployment separation: Task 8.
- No unresolved placeholders remain in this plan.
- Type names are consistent:
  - `DispatchEventQuery`
  - `DispatchEventRepository`
  - `DispatchEventSummary`
  - `DispatchEventPage`
  - `NewGormDispatchEventRepository`
  - `NewDispatchEventHandler`
- Scope remains lightweight: no export, alerting, remediation, or chart library.
