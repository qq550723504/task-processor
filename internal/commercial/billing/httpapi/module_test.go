package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
)

func TestBodyConsumingPOSTRoutesUseBoundedBodyRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes, err := Routes(NewHandler(nil))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, route := range routes {
		if route.Method == http.MethodPost && (route.Path == quotePath || route.Path == orderPath || route.Path == subscriptionQuotePath || route.Path == subscriptionOrderPath) {
			seen[route.Path] = true
			if route.RejectUnreadRequestBody || route.Handler == nil {
				t.Errorf("body-consuming route %s must not reject before handler read", route.Path)
			}
		}
	}
	if !seen[quotePath] || !seen[orderPath] || !seen[subscriptionQuotePath] || !seen[subscriptionOrderPath] {
		t.Fatalf("expected quote and order body-consuming routes, got %#v", seen)
	}
}

func TestSubscriptionPurchaseRoutesAreDedicated(t *testing.T) {
	routes, err := Routes(NewHandler(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		http.MethodGet + " " + subscriptionOfferPath:       authz.PermissionWorkbenchCommercialRead,
		http.MethodPost + " " + subscriptionQuotePath:      authz.PermissionWorkbenchCommercialPurchase,
		http.MethodPost + " " + subscriptionOrderPath:      authz.PermissionWorkbenchCommercialPurchase,
		http.MethodGet + " " + subscriptionOrderDetailPath: authz.PermissionWorkbenchCommercialRead,
	}
	seen := map[string]bool{}
	for _, route := range routes {
		key := route.Method + " " + route.Path
		permission, ok := want[key]
		if ok {
			seen[key] = true
			if route.Permission != permission || route.AuthPolicy != httproute.AuthPolicyCurrentIdentity || route.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite {
				t.Errorf("route %s policy = permission %q, auth %q, organization %q", key, route.Permission, route.AuthPolicy, route.OrganizationAccessPolicy)
			}
		}
	}
	for route := range want {
		if !seen[route] {
			t.Errorf("missing route %s", route)
		}
	}
}

func TestSubscriptionWritesRejectBrowserOwnedOrganizationAndPlanFacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, scenario := range map[string]struct {
		path   string
		body   string
		invoke func(*Handler, *gin.Context)
	}{
		"quote organization": {subscriptionQuotePath, `{"offer_id":"offer-1","organization_id":"other"}`, (*Handler).CreateSubscriptionQuote},
		"quote plan":         {subscriptionQuotePath, `{"offer_id":"offer-1","plan_code":"professional"}`, (*Handler).CreateSubscriptionQuote},
		"order actor":        {subscriptionOrderPath, `{"quote_id":"quote-1","actor_id":"other"}`, (*Handler).CreateSubscriptionOrder},
		"order amount":       {subscriptionOrderPath, `{"quote_id":"quote-1","amount_minor":"0"}`, (*Handler).CreateSubscriptionOrder},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			request := httptest.NewRequest(http.MethodPost, scenario.path, strings.NewReader(scenario.body))
			request.Header.Set("Idempotency-Key", "test-key")
			request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor-1", TenantID: "org-1", EffectiveOrganizationID: "org-1"}))
			ctx.Request = request
			scenario.invoke(NewHandler(&billing.Service{}), ctx)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestSubscriptionOrderRequiresExactlyOneIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	request := httptest.NewRequest(http.MethodPost, subscriptionOrderPath, strings.NewReader(`{"quote_id":"quote-1"}`))
	request.Header.Add("Idempotency-Key", "key-a")
	request.Header.Add("Idempotency-Key", "key-b")
	request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor-1", TenantID: "org-1", EffectiveOrganizationID: "org-1"}))
	ctx.Request = request
	NewHandler(&billing.Service{}).CreateSubscriptionOrder(ctx)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestOrdersAcceptsSubscriptionKindAndProductFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"kind=SUBSCRIPTION_PURCHASE", "product_kind=SUBSCRIPTION_PLAN"} {
		t.Run(query, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			request := httptest.NewRequest(http.MethodGet, orderPath+"?"+query, nil)
			request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor-1", TenantID: "org-1", EffectiveOrganizationID: "org-1"}))
			ctx.Request = request
			NewHandler(&billing.Service{}).Orders(ctx)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("subscription filter was rejected before order read: status = %d, body = %s", response.Code, response.Body.String())
			}
		})
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
