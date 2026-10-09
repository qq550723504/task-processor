package ecoservices

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
)

type closedPollingProvider struct {
	unpaidPollingProvider
	queries int
}

func (p *closedPollingProvider) QueryServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	p.queries++
	result, err := p.unpaidPollingProvider.QueryServicePayment(ctx, o)
	result.State, result.EventID = "CLOSED", "natural-close:"+o.TradeNo
	return result, err
}

func TestNaturalClosureTerminatesOriginalRecoveryWithoutCancellationIdentity(t *testing.T) {
	r, _, m, _, b, _, _ := fulfillmentFundsFixture(t)
	req, original := checkoutAdmissionFixture(t, r)
	ctx := context.Background()
	provider := &closedPollingProvider{}
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
		page, err := r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
		if err != nil || len(page.Requests) != 1 {
			t.Fatalf("read original: %+v %v", page, err)
		}
		return page.Requests[0]
	}
	closed := read()
	if closed.State != "CANCELLED" || closed.FinancialState != "CLOSED_UNPAID" || closed.PaymentReceiptID != "" {
		t.Fatalf("verified close left pending page: %+v", closed)
	}
	var row financialRow
	if err := r.db.Where("id=?", original.ID).Take(&row).Error; err != nil || row.State != "DONE" {
		t.Fatalf("original create not terminal: %+v %v", row, err)
	}
	for i := 0; i < 3; i++ {
		if err := service.Recover(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if pending, err := r.PendingFinancialCommands(ctx, 20); err != nil || len(pending) != 0 || provider.queries != 1 {
		t.Fatalf("closed trade keeps polling: %+v %v queries=%d", pending, err, provider.queries)
	}
	// The trusted notification wakes only the original CREATE. A real late
	// payment is retained, without inventing a cancellation/refund request.
	o, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	paid := billing.ServicePaymentObservation{EventID: "late-paid:" + o.TradeNo, ProfileVersion: o.Profile.Version, PlatformMerchantID: o.Profile.PlatformMerchantID, AppID: o.Profile.AppID, ProviderMerchantID: o.Source.ProviderMerchantID, TradeNo: o.TradeNo, Currency: "CNY", State: "PAID", VerificationVersion: "verified-fixture", TransactionID: "late-original-payment", AmountMinor: o.Source.AmountMinor, OccurredAt: time.Now().UTC()}
	if err := b.RecordServicePaymentObservation(ctx, o, paid); err != nil {
		t.Fatal(err)
	}
	if err := r.WakeOriginalServicePurchase(ctx, req.OrderID); err != nil {
		t.Fatal(err)
	}
	if err := service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	late := read()
	if late.State != "CANCELLED" || late.FinancialState != "RECONCILIATION_REQUIRED" || late.FinancialReason != "PAYMENT_AFTER_UNPAID_CLOSE" || !late.FinancialFence || late.PaymentReceiptID == "" {
		t.Fatalf("late payment reopened service or was lost: %+v", late)
	}
	if err := r.CompleteFinancialCommand(ctx, original, e.FinancialResult{OrderID: req.OrderID, State: "CLOSED_UNPAID", ReceiptID: "older-close", Revision: closed.FinancialRevision}); err != nil {
		t.Fatal(err)
	}
	if after := read(); after.State != late.State || after.FinancialState != late.FinancialState || after.PaymentReceiptID != late.PaymentReceiptID {
		t.Fatalf("old close replaced late payment: %+v", after)
	}
	var count int64
	if err := r.db.Model(&financialRow{}).Where("order_id=?", req.OrderID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("natural closure invented cancellation: count=%d %v", count, err)
	}
	view, err := m.ReadServiceFunds(ctx, req.OrderID)
	current, _ := b.ReadServicePurchase(ctx, req.OrderID)
	receipt, receiptErr := m.ReadServicePayment(ctx, current.MoneyInput())
	if err != nil || receiptErr != nil || receipt.ReceiptID != late.PaymentReceiptID || current.Payment == nil || current.Payment.TransactionID != paid.TransactionID || view.RefundedMinor != 0 || view.SharedMinor != 0 || view.PendingOperationID != "" {
		t.Fatalf("late original funds lost or mutated: %+v %v", view, err)
	}
}
