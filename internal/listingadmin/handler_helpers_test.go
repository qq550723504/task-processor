package listingadmin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindJSONWritesInvalidRequestResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	ctx.Request.Header.Set("Content-Type", "application/json")

	var payload struct {
		Name string `json:"name"`
	}

	if bindJSON(ctx, &payload) {
		t.Fatal("expected bindJSON to fail for invalid json")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"invalid_request"`) {
		t.Fatalf("body = %s, want invalid_request", recorder.Body.String())
	}
}

func TestWriteMappedHandlerErrorUsesSpecificRuleBeforeFallback(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeMappedHandlerError(ctx, ErrCategoryHasChildren, "category_delete_failed",
		handlerErrorRule{match: ErrCategoryNotFound, status: http.StatusNotFound, errorCode: "category_not_found"},
		handlerErrorRule{match: ErrCategoryHasChildren, status: http.StatusConflict, errorCode: "category_has_children"},
	)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"category_has_children"`) {
		t.Fatalf("body = %s, want category_has_children", recorder.Body.String())
	}
}

func TestWriteMappedHandlerErrorFallsBackToInternalError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeMappedHandlerError(ctx, errors.New("boom"), "store_update_failed",
		handlerErrorRule{match: ErrStoreNotFound, status: http.StatusNotFound, errorCode: "store_not_found"},
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"store_update_failed"`) {
		t.Fatalf("body = %s, want store_update_failed", recorder.Body.String())
	}
}

func TestWriteValidationErrorWritesBadRequestPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeValidationError(ctx, "invalid_store", errors.New("store is invalid"))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"invalid_store"`) {
		t.Fatalf("body = %s, want invalid_store", recorder.Body.String())
	}
}

func TestWriteInternalHandlerErrorWritesInternalErrorPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeInternalHandlerError(ctx, "store_list_failed", errors.New("boom"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"store_list_failed"`) {
		t.Fatalf("body = %s, want store_list_failed", recorder.Body.String())
	}
}

func TestRequestPageParamsSupportsLegacyAndCurrentKeys(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?pageNo=3&pageSize=55", nil)

	page, pageSize := requestPageParams(ctx)
	if page != 3 || pageSize != 55 {
		t.Fatalf("page/pageSize = %d/%d, want 3/55", page, pageSize)
	}
}

func TestRequestListScopeBuildsTenantOwnerAndPagingContext(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?page=2&page_size=30", nil)
	ctx.Request.Header.Set("X-Tenant-ID", "101")
	ctx.Request.Header.Set("X-User-ID", "user-101")

	scope := requestListScope(ctx)
	if scope.TenantID != 101 {
		t.Fatalf("tenantID = %d, want 101", scope.TenantID)
	}
	if scope.OwnerUserID != "user-101" {
		t.Fatalf("ownerUserID = %q, want user-101", scope.OwnerUserID)
	}
	if scope.Page != 2 || scope.PageSize != 30 {
		t.Fatalf("page/pageSize = %d/%d, want 2/30", scope.Page, scope.PageSize)
	}
}

func TestApplyListQueryScopeSetsSharedFields(t *testing.T) {
	t.Parallel()

	scope := listQueryScope{
		TenantID:    101,
		OwnerUserID: "user-101",
		Page:        3,
		PageSize:    25,
	}

	storeQuery := StoreQuery{}
	applyListQueryScope(&storeQuery, scope)
	if storeQuery.TenantID != 101 || storeQuery.OwnerUserID != "user-101" || storeQuery.Page != 3 || storeQuery.PageSize != 25 {
		t.Fatalf("storeQuery = %+v, want shared scope fields applied", storeQuery)
	}

	categoryQuery := CategoryQuery{}
	applyListQueryScope(&categoryQuery, scope)
	if categoryQuery.TenantID != 101 || categoryQuery.OwnerUserID != "user-101" {
		t.Fatalf("categoryQuery = %+v, want shared scope fields applied", categoryQuery)
	}
}

func TestRequestTenantIDFallsBackToTenantQueryParam(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?tenant_id=303", nil)

	if tenantID := requestTenantID(ctx); tenantID != 303 {
		t.Fatalf("tenantID = %d, want 303", tenantID)
	}
}

func TestPathIDWritesInvalidIDError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}

	if _, ok := pathID(ctx); ok {
		t.Fatal("expected pathID to fail for invalid value")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"invalid_id"`) {
		t.Fatalf("body = %s, want invalid_id", recorder.Body.String())
	}
}
