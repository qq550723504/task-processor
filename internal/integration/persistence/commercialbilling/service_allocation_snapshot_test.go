package commercialbilling

import (
	"context"
	"task-processor/internal/commercial/billing"
	"testing"
)

func TestServiceOrderAllocationSurvivesSettlementAndRefund(t *testing.T) {
	svc, store, funds, source, provider := servicePurchaseFixture(t)
	source.original.Allocation.CommissionBPS = 2000
	provider.paid = true
	ctx := context.Background()
	if _, err := svc.Execute(ctx, source.original); err != nil {
		t.Fatal(err)
	}
	saved, err := store.ReadServicePurchase(ctx, source.original.OrderID)
	if err != nil || saved.Source.Allocation != source.original.Allocation || saved.MoneyInput().Allocation != source.original.Allocation {
		t.Fatalf("order lost source snapshot: %+v %v", saved.Source, err)
	}
	changed := source.original
	changed.Allocation.CommissionBPS = 1000
	if _, err := svc.Execute(ctx, changed); err != billing.ErrConflict {
		t.Fatalf("original fee overwritten: %v", err)
	}
	if result, err := svc.Execute(ctx, serviceCommand(source, "SETTLE", "accepted", 101)); err != nil || result.State != "SETTLED" {
		t.Fatalf("settlement failed %+v %v", result, err)
	}
	if result, err := svc.Execute(ctx, serviceCommand(source, "REFUND", "refund", 2)); err != nil || result.State != "REFUNDED" {
		t.Fatalf("refund failed %+v %v", result, err)
	}
	f, err := funds.ReadServiceFunds(ctx, source.original.OrderID)
	if err != nil || f.SharedMinor != 20 || f.ReturnedMinor != 1 || f.PlatformMinor != 19 || f.ProviderMinor != 80 {
		t.Fatalf("wrong original-rate funds %+v %v", f, err)
	}
}
