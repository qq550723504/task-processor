package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestImportTaskHandlerListsTasksWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	seedImportTask(t, router.db, listingProductImportTask{
		TenantID:   101,
		StoreID:    1,
		Platform:   "Amazon",
		Region:     "US",
		CategoryID: int64PtrIfPositive(10),
		ProductID:  "B001",
		Status:     0,
		Priority:   5,
	})
	seedImportTask(t, router.db, listingProductImportTask{
		TenantID:   202,
		StoreID:    2,
		Platform:   "Amazon",
		Region:     "US",
		CategoryID: int64PtrIfPositive(10),
		ProductID:  "B002",
		Status:     0,
		Priority:   5,
	})

	req := httptest.NewRequest(http.MethodGet, "/import-tasks?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /import-tasks = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page ImportTaskPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one task", page)
	}
	if page.Items[0].ProductID != "B001" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 task only", page.Items)
	}
}

func TestImportTaskHandlerRejectsInvalidNumericFilters(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/import-tasks?storeId=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /import-tasks invalid filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_store_id"`) {
		t.Fatalf("response body = %s, want invalid_store_id", resp.Body.String())
	}
}

func TestImportTaskHandlerBatchCreatesTasksWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	body := bytes.NewBufferString(`{
		"storeId": 11,
		"categoryId": 22,
		"platform": "Amazon",
		"targetPlatform": "SHEIN",
		"region": "US",
		"priority": 8,
		"productIds": ["B001", "B002", "B001", " "]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /import-tasks/batch = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created BatchCreateImportTaskResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.CreatedCount != 2 || len(created.Items) != 2 {
		t.Fatalf("created = %+v, want two unique tasks", created)
	}
	for _, item := range created.Items {
		if item.TenantID != 303 || item.StoreID == nil || *item.StoreID != 11 || item.Status != 0 || item.Priority != 8 {
			t.Fatalf("item = %+v, want request tenant and defaults", item)
		}
		if item.Platform != "amazon" || item.SourcePlatform != "amazon" || item.TargetPlatform != "shein" {
			t.Fatalf("item platforms = %q/%q/%q, want amazon/amazon/shein", item.Platform, item.SourcePlatform, item.TargetPlatform)
		}
	}
}

func TestImportTaskHandlerBatchCreatesTasksWithoutCategory(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	body := bytes.NewBufferString(`{
		"storeId": 11,
		"platform": "Amazon",
		"region": "US",
		"priority": 8,
		"productIds": ["B001"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /import-tasks/batch without category = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"categoryId":null`) {
		t.Fatalf("response body = %s, want categoryId null", resp.Body.String())
	}
	var created BatchCreateImportTaskResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(created.Items) != 1 || created.Items[0].CategoryID != nil {
		t.Fatalf("created = %+v, want one task with nil category", created)
	}
	if created.Items[0].TargetPlatform != "amazon" {
		t.Fatalf("targetPlatform = %q, want legacy platform amazon", created.Items[0].TargetPlatform)
	}
}

func TestImportTaskHandlerBatchRejectsExistingActiveTaskWithConflict(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	seedImportTask(t, router.db, listingProductImportTask{
		TenantID:       303,
		StoreID:        11,
		Platform:       "amazon",
		SourcePlatform: "amazon",
		TargetPlatform: "shein",
		Region:         "US",
		ProductID:      "B001",
		Deleted:        0,
	})

	body := bytes.NewBufferString(`{
		"storeId": 11,
		"platform": "Amazon",
		"targetPlatform": "SHEIN",
		"region": "US",
		"productIds": ["B001"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("POST /import-tasks/batch duplicate = %d, body=%s; want 409", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"import_task_already_exists"`) {
		t.Fatalf("response body = %s, want import_task_already_exists", resp.Body.String())
	}
}

func TestImportTaskHandlerBatchRejectsExistingActiveTaskWithCanonicalTargetPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		existingPlatform string
		existingTarget   string
		requestPlatform  string
		requestTarget    string
	}{
		{
			name:             "mixed case target",
			existingPlatform: "amazon",
			existingTarget:   "SHEIN",
			requestPlatform:  "Amazon",
			requestTarget:    "shein",
		},
		{
			name:             "blank target falls back to platform",
			existingPlatform: "Amazon",
			existingTarget:   "",
			requestPlatform:  "amazon",
			requestTarget:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := newImportTaskTestRouter(t)
			seedImportTask(t, router.db, listingProductImportTask{
				TenantID:       303,
				StoreID:        11,
				Platform:       tc.existingPlatform,
				SourcePlatform: tc.existingPlatform,
				TargetPlatform: tc.existingTarget,
				Region:         "US",
				ProductID:      "B001",
				Deleted:        0,
			})

			body := bytes.NewBufferString(`{"storeId":11,"platform":"` + tc.requestPlatform + `","targetPlatform":"` + tc.requestTarget + `","region":"US","productIds":["B001"]}`)
			req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
			req.Header.Set("X-Tenant-ID", "303")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.engine.ServeHTTP(resp, req)

			if resp.Code != http.StatusConflict {
				t.Fatalf("POST /import-tasks/batch canonical duplicate = %d, body=%s; want 409", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestImportTaskHandlerAllowsSameImportTaskTupleAcrossTenants(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	seedImportTask(t, router.db, listingProductImportTask{
		TenantID:       303,
		StoreID:        11,
		Platform:       "amazon",
		SourcePlatform: "amazon",
		TargetPlatform: "shein",
		Region:         "US",
		ProductID:      "B001",
		Deleted:        0,
	})

	body := bytes.NewBufferString(`{"storeId":11,"platform":"Amazon","targetPlatform":"SHEIN","region":"US","productIds":["B001"]}`)
	req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
	req.Header.Set("X-Tenant-ID", "404")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /import-tasks/batch cross-tenant duplicate = %d, body=%s; want 201", resp.Code, resp.Body.String())
	}
}

func TestImportTaskHandlerRejectsWritesWhileCanonicalDuplicatesNeedRemediation(t *testing.T) {
	router := newImportTaskTestRouter(t)
	for _, target := range []string{"SHEIN", "shein"} {
		seedImportTask(t, router.db, listingProductImportTask{
			TenantID:       303,
			StoreID:        11,
			Platform:       "amazon",
			SourcePlatform: "amazon",
			TargetPlatform: target,
			Region:         "US",
			ProductID:      "B001",
			Deleted:        0,
		})
	}
	if err := AutoMigrateImportTaskRepository(router.db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() with canonical duplicates = %v", err)
	}

	body := bytes.NewBufferString(`{"storeId":11,"platform":"Amazon","targetPlatform":"SHEIN","region":"US","productIds":["B002"]}`)
	req := httptest.NewRequest(http.MethodPost, "/import-tasks/batch", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /import-tasks/batch with legacy duplicates = %d, body=%s; want 503", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"import_task_integrity_unavailable"`) {
		t.Fatalf("response body = %s, want import_task_integrity_unavailable", resp.Body.String())
	}
}

func TestImportTaskHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newImportTaskTestRouter(t)
	task := seedImportTask(t, router.db, listingProductImportTask{
		TenantID:   404,
		StoreID:    1,
		Platform:   "Amazon",
		Region:     "US",
		CategoryID: int64PtrIfPositive(10),
		ProductID:  "B001",
		Status:     0,
	})

	req := httptest.NewRequest(http.MethodDelete, "/import-tasks/1", nil)
	req.Header.Set("X-Tenant-ID", "404")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /import-tasks/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingProductImportTask
	if err := router.db.Table("listing_product_import_task").Where("id = ?", task.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newImportTaskTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate listing_product_import_task: %v", err)
	}
	repo := NewGormImportTaskRepository(router.db)
	handler := NewImportTaskHandler(repo)
	router.engine.GET("/import-tasks", handler.ListImportTasks)
	router.engine.POST("/import-tasks/batch", handler.BatchCreateImportTasks)
	router.engine.DELETE("/import-tasks/:id", handler.DeleteImportTask)
	return router
}

func seedImportTask(t *testing.T, db *gorm.DB, task listingProductImportTask) listingProductImportTask {
	t.Helper()
	if err := db.Table("listing_product_import_task").Create(&task).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}
	return task
}
