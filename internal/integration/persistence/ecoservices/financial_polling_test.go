package ecoservices

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
)

type unpaidPollingProvider struct{ dormantCheckoutProvider }

func (*unpaidPollingProvider) QueryServicePayment(_ context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	return billing.ServicePaymentObservation{EventID: "unpaid:" + o.TradeNo, ProfileVersion: o.Profile.Version, PlatformMerchantID: o.Profile.PlatformMerchantID, AppID: o.Profile.AppID, ProviderMerchantID: o.Source.ProviderMerchantID, TradeNo: o.TradeNo, Currency: "CNY", State: "UNPAID", VerificationVersion: "verified-fixture"}, nil
}

func TestUnpaidRecoveryPollingKeepsDisplayedCancellationVersion(t *testing.T) {
	for _, operation := range []string{"read-version", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			r, _, m, _, b, _, _ := fulfillmentFundsFixture(t)
			req, original := checkoutAdmissionFixture(t, r)
			ctx := context.Background()
			provider := &unpaidPollingProvider{}
			purchases, err := billing.NewServicePurchases(b, m, provider, ecoservicesbilling.Source{Repository: r, Authorizer: originalBuyerCheckoutAuthorization{}}, dormantCheckoutProtection{})
			if err != nil {
				t.Fatal(err)
			}
			service, err := e.NewService(r, ecoservicesbilling.Trading{Purchases: purchases}, 180)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := purchases.Checkout(ctx, "buyer", "buyer", req.OrderID); err != nil {
				t.Fatal(err)
			}
			if err := service.Recover(ctx); err != nil {
				t.Fatal(err)
			}
			read := func() e.Request {
				t.Helper()
				page, err := service.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
				if err != nil || len(page.Requests) != 1 {
					t.Fatalf("read original request: %+v %v", page, err)
				}
				return page.Requests[0]
			}
			shown := read()
			before, err := b.ReadServicePurchase(ctx, req.OrderID)
			if err != nil || !before.PaymentDispatched || before.PaymentReceiptID != "" {
				t.Fatalf("original unpaid checkout missing: %+v %v", before, err)
			}
			for i := 0; i < 3; i++ {
				// Make the original command due, as each recovery tick would;
				// no sleep or replacement order/financial command is needed.
				if err := r.db.Model(&financialRow{}).Where("id=?", original.ID).Update("next_attempt_at", time.Time{}).Error; err != nil {
					t.Fatal(err)
				}
				if err := service.Recover(ctx); err != nil {
					t.Fatal(err)
				}
			}
			after, err := b.ReadServicePurchase(ctx, req.OrderID)
			if err != nil || after.Version <= before.Version || after.LeaseToken != "" || provider.creates != 1 {
				t.Fatalf("worker fencing/original dispatch changed: %+v %v creates=%d", after, err, provider.creates)
			}
			var row financialRow
			if err := r.db.Where("id=?", original.ID).Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			var result e.FinancialResult
			if err := json.Unmarshal(row.Result, &result); err != nil || result.Revision != after.Version {
				t.Fatalf("latest original poll result not durable: %+v %v", result, err)
			}
			current := read()
			if current.FinancialRevision != after.Version || current.FinancialRevision <= shown.FinancialRevision {
				t.Fatalf("latest stale-result floor not durable: %+v", current)
			}
			if operation == "read-version" {
				if current.Version != shown.Version || !current.UpdatedAt.Equal(shown.UpdatedAt) {
					t.Fatalf("lease-only polling changed displayed request: before version=%d time=%s; after version=%d time=%s", shown.Version, shown.UpdatedAt, current.Version, current.UpdatedAt)
				}
				return
			}
			cancelled, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "cancel", ID: req.ID, Version: shown.Version})
			if err != nil || cancelled.Request == nil || cancelled.Request.State != "CANCEL_REQUESTED" {
				t.Fatalf("poll invalidated displayed cancellation version %d: %+v %v", shown.Version, cancelled, err)
			}
		})
	}
}

func TestFinancialPollingFloorPersistsWithoutReopeningReconciliation(t *testing.T) {
	r, service := fixture(t)
	req, original := checkoutAdmissionFixture(t, r)
	ctx := context.Background()
	result := e.FinancialResult{OrderID: req.OrderID, PaymentReceiptID: "original-payment", State: "RECONCILIATION_REQUIRED", Reason: "verified-chargeback", Revision: 10}
	if err := r.CompleteFinancialCommand(ctx, original, result); err != nil {
		t.Fatal(err)
	}
	read := func() e.Request {
		t.Helper()
		page, err := service.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
		if err != nil || len(page.Requests) != 1 {
			t.Fatalf("read projected request: %+v %v", page, err)
		}
		return page.Requests[0]
	}
	before := read()
	result.Revision = 20
	if err := r.CompleteFinancialCommand(ctx, original, result); err != nil {
		t.Fatal(err)
	}
	after := read()
	if after.FinancialRevision != 20 || after.Version != before.Version || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("metadata-only floor changed visible request or was lost: before=%+v after=%+v", before, after)
	}
	if err := r.CompleteFinancialCommand(ctx, original, e.FinancialResult{OrderID: req.OrderID, PaymentReceiptID: "original-payment", State: "PAID", Revision: 15}); err != nil {
		t.Fatal(err)
	}
	after = read()
	if after.FinancialRevision != 20 || after.FinancialState != "RECONCILIATION_REQUIRED" || !after.FinancialFence || after.Version != before.Version || after.State != "ORDER_PENDING" {
		t.Fatalf("older paid worker replaced durable reconciliation: %+v", after)
	}
}
