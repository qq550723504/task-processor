package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestOrderResponseIncludesCanonicalDescription(t *testing.T) {
	response := orderResponseFromDomain(billing.Order{OrderID: "order-1", Description: "AI 点数 × 10"})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["description"] != "AI 点数 × 10" {
		t.Fatalf("order response description = %#v; want canonical description", payload["description"])
	}
}

func TestChunkedWriteRequestGetsExplicitInvalidRequestResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	request := httptest.NewRequest(http.MethodPost, quotePath, strings.NewReader(`{"offer_id":"offer-1","quantity":"1"}`))
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	ctx.Request = request

	if _, ok := writeOrganization(ctx); ok {
		t.Fatal("chunked write request must be rejected before service execution")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusBadRequest)
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Code != "INVALID_REQUEST" {
		t.Fatalf("payload=%s err=%v; want INVALID_REQUEST", response.Body.String(), err)
	}
}
