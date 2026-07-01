package listingadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScheduledTaskConfigHandlerUpsertsAndListsWithinTenant(t *testing.T) {
	t.Parallel()

	router := newScheduledTaskConfigTestRouter(t)
	body := bytes.NewBufferString(`{
		"storeId": 962,
		"platform": "SHEIN",
		"taskType": "inventory",
		"enabled": true,
		"intervalSeconds": 3600,
		"remark": "enable 962 inventory"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/scheduled-task-configs", body)
	req.Header.Set("X-Tenant-ID", "246")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("POST /scheduled-task-configs = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created ScheduledTaskConfig
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == 0 || created.TenantID != 246 || created.StoreID != 962 || created.Platform != "shein" || !created.Enabled {
		t.Fatalf("created = %+v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/scheduled-task-configs?platform=SHEIN&taskType=inventory", nil)
	req.Header.Set("X-Tenant-ID", "246")
	resp = httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /scheduled-task-configs = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page ScheduledTaskConfigPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].StoreID != 962 {
		t.Fatalf("page = %+v", page)
	}
}

func TestScheduledTaskConfigHandlerUpdatesStatus(t *testing.T) {
	t.Parallel()

	router := newScheduledTaskConfigTestRouter(t)
	created, err := NewGormScheduledTaskConfigRepository(router.db).UpsertScheduledTaskConfig(
		contextWithTenantForScheduledTaskTest(),
		&ScheduledTaskConfig{
			TenantID:        246,
			StoreID:         962,
			Platform:        "shein",
			TaskType:        "inventory",
			Enabled:         true,
			IntervalSeconds: 3600,
		},
	)
	if err != nil {
		t.Fatalf("seed scheduled task config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/scheduled-task-configs/1/status", bytes.NewBufferString(`{"enabled":false,"remark":"pause"}`))
	req.Header.Set("X-Tenant-ID", "246")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /scheduled-task-configs/1/status = %d, body=%s", resp.Code, resp.Body.String())
	}
	var updated ScheduledTaskConfig
	if err := json.Unmarshal(resp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.ID != created.ID || updated.Enabled || updated.Remark != "pause" {
		t.Fatalf("updated = %+v, created ID=%d", updated, created.ID)
	}
}

func TestScheduledTaskConfigHandlerRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	router := newScheduledTaskConfigTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/scheduled-task-configs?storeId=abc", nil)
	req.Header.Set("X-Tenant-ID", "246")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET invalid storeId = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_store_id"`) {
		t.Fatalf("body = %s, want invalid_store_id", resp.Body.String())
	}
}

func newScheduledTaskConfigTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := AutoMigrateScheduledTaskConfigRepository(router.db); err != nil {
		t.Fatalf("migrate scheduled task config: %v", err)
	}
	handler := NewScheduledTaskConfigHandler(NewGormScheduledTaskConfigRepository(router.db))
	router.engine.GET("/scheduled-task-configs", handler.ListScheduledTaskConfigs)
	router.engine.POST("/scheduled-task-configs", handler.UpsertScheduledTaskConfig)
	router.engine.PATCH("/scheduled-task-configs/:id/status", handler.UpdateScheduledTaskConfigStatus)
	return router
}

func contextWithTenantForScheduledTaskTest() context.Context {
	return context.Background()
}
