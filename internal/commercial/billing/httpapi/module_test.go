package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"task-processor/internal/commercial/billing"
)

func TestBodyConsumingPOSTRoutesUseBoundedBodyRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes, err := Routes(NewHandler(nil))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, route := range routes {
		if route.Method == http.MethodPost && (route.Path == quotePath || route.Path == orderPath) {
			seen[route.Path] = true
			if route.RejectUnreadRequestBody || route.Handler == nil {
				t.Errorf("body-consuming route %s must not reject before handler read", route.Path)
			}
		}
	}
	if !seen[quotePath] || !seen[orderPath] {
		t.Fatalf("expected quote and order body-consuming routes, got %#v", seen)
	}
}

func TestNotFoundMapsToNotFoundResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, orderPath+"/missing", nil)

	writeServiceError(ctx, billing.ErrNotFound)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusNotFound)
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Code != "NOT_FOUND" {
		t.Fatalf("payload=%s err=%v; want NOT_FOUND", response.Body.String(), err)
	}
}

func TestUnavailableOfferMapsToOfferUnavailableResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, quotePath, nil)

	writeServiceError(ctx, billing.ErrOfferUnavailable)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusServiceUnavailable)
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Code != "OFFER_UNAVAILABLE" {
		t.Fatalf("payload=%s err=%v; want OFFER_UNAVAILABLE", response.Body.String(), err)
	}
}
