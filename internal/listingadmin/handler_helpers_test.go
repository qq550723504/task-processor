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
