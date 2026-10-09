package commercialbilling

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/ledger/money"
)

func TestOriginalPurchaseReadProjectsChargebackAndKeepsInFlightRecovery(t *testing.T) {
	svc, store, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	p.paidAt = time.Now().UTC().Add(-time.Hour)
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	p.lostEffect = true
	settle := serviceCommand(src, "SETTLE", "original-in-flight-settle", 101)
	if _, err := svc.Execute(ctx, settle); err == nil {
		t.Fatal("lost original share response not exercised")
	}
	before, err := store.ReadServicePurchase(ctx, src.original.OrderID)
	if err != nil || before.Operation == nil || !before.Operation.Dispatched {
		t.Fatal("original in-flight operation missing", err)
	}
	if err := funds.RecordChargebackSettlement(ctx, money.ChargebackSettlement{ChargebackID: "signed-chargeback", PaymentID: before.MoneyInput().Payment.PaymentID, AmountMinor: 101, OccurredAt: time.Now().UTC(), ProviderReference: "verified-original-chargeback"}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Execute(ctx, src.original)
	if err != nil || result.State != "RECONCILIATION_REQUIRED" || result.Reason != "CHANNEL_CHARGEBACK_REQUIRES_RECONCILIATION" {
		t.Fatalf("cached original purchase hid current funds: %+v %v", result, err)
	}
	current, err := store.ReadServicePurchase(ctx, src.original.OrderID)
	if err != nil || current.Operation == nil || current.Operation.ProviderRequestID != before.Operation.ProviderRequestID || current.ActiveCommand.ID != settle.ID {
		t.Fatal("funds read destroyed original recovery", err)
	}
	if _, err := svc.Execute(ctx, settle); err != nil {
		t.Fatal(err)
	}
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.SharedMinor != 10 || f.ChargedBackMinor != 101 || f.ReconciliationReason == "" {
		t.Fatalf("original share/chargeback facts lost: %+v %v", f, err)
	}
	if len(p.dispatched) != 1 || p.dispatched[before.Operation.ProviderRequestID] != 1 || p.unsplitQueries != 0 {
		t.Fatalf("readback dispatched a new effect or expired query: %+v queries=%d", p.dispatched, p.unsplitQueries)
	}
}
