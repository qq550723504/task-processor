package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestSubscriptionGuardKeepsLegacyCounterPathWhenLedgerIsConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := listingsubscription.NewMemRepository()
	ledger := listingsubscription.NewMemUsageLedger(repo)
	service, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := service.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleListingKit, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"listingkit_generations_succeeded": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}

	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: service}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/listingkit", nil)
	c.Request.Header.Set("X-Tenant-ID", listingkit.DefaultTenantID)
	if !h.requireSubscriptionUsage(c, listingsubscription.ModuleListingKit, "listingkit_generations_succeeded", 1) {
		t.Fatal("requireSubscriptionUsage() = false, want legacy usage guard to allow")
	}

	summary, err := service.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	for _, entitlement := range summary.Entitlements {
		if entitlement.Module.Code == listingsubscription.ModuleListingKit {
			if entitlement.Used["listingkit_generations_succeeded"] != 1 {
				t.Fatalf("ListingKit generation counter = %d, want 1", entitlement.Used["listingkit_generations_succeeded"])
			}
			break
		}
	}
	pending, err := service.ListPendingUsageOutbox(t.Context(), 10)
	if err != nil {
		t.Fatalf("ListPendingUsageOutbox() error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("paid entrypoint created ledger outbox items = %#v, want none", pending)
	}
}
