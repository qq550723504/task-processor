package commercialbilling

import (
	"context"
	"reflect"
	"testing"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
)

func TestServiceFailedRefundAfterReturnFencesOriginalAndKeepsProof(t *testing.T) {
	for _, lostSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "lost-B7"}[lostSave], func(t *testing.T) {
			svc, store, funds, src, provider := servicePurchaseFixture(t)
			if lostSave {
				var err error
				svc, err = billing.NewServicePurchases(&loseFailedBillingSave{Repository: store}, funds, provider, src, serviceProtectionFixture{})
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			src.original.AmountMinor = 100
			provider.paid = true
			if _, err := svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			if result, err := svc.Execute(ctx, serviceCommand(src, "SETTLE", "accepted", 100)); err != nil || result.State != "SETTLED" {
				t.Fatal("settlement did not complete", result, err)
			}
			provider.failKind = money.ServiceRefund
			original := serviceCommand(src, "REFUND", "failed-fifty", 50)
			result, err := svc.Execute(ctx, original)
			if lostSave {
				if err == nil {
					t.Fatal("B7 failure was not exercised")
				}
				result, err = svc.Execute(ctx, original)
			}
			if err != nil || result.State != "RECONCILIATION_REQUIRED" || result.ReceiptID == "" {
				t.Fatalf("failed refund lost original proof or funds fence: %+v %v", result, err)
			}
			f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
			if err != nil || f.ReturnedMinor != 5 || f.RefundedMinor != 0 || f.PendingOperationID != "" {
				t.Fatal("original return or terminal failure was lost", f, err)
			}
			before := len(provider.dispatched)
			provider.failKind = ""
			again, err := svc.Execute(ctx, original)
			if err != nil || !reflect.DeepEqual(again, result) {
				t.Fatal("original terminal failure did not replay exactly", again, result, err)
			}
			for _, kind := range []string{"REFUND", "SETTLE"} {
				amount := int64(20)
				if kind == "SETTLE" {
					amount = 100
				}
				blocked, err := svc.Execute(ctx, serviceCommand(src, kind, "later-"+kind, amount))
				if err != nil || blocked.State != "RECONCILIATION_REQUIRED" || len(provider.dispatched) != before {
					t.Fatalf("new %s effect misattributed returned commission: %+v %v", kind, blocked, err)
				}
			}
			for id, count := range provider.dispatched {
				if count != 1 {
					t.Fatalf("repeated original effect %s: %d", id, count)
				}
			}
		})
	}
}

func TestServiceFailedRefundReleaseAfterReturnKeepsReconciliationFence(t *testing.T) {
	svc, _, funds, src, provider := servicePurchaseFixture(t)
	ctx := context.Background()
	src.original.AmountMinor = 100
	provider.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	provider.failKind = money.ServiceFinish
	if result, err := svc.Execute(ctx, serviceCommand(src, "SETTLE", "failed-finish", 100)); err != nil || result.State != "CHANNEL_OPERATION_FAILED" {
		t.Fatal("finish failure did not retain successful share", result, err)
	}
	provider.failKind = money.ServiceRefundRelease
	result, err := svc.Execute(ctx, serviceCommand(src, "REFUND", "failed-release-fifty", 50))
	if err != nil || result.State != "RECONCILIATION_REQUIRED" || result.ReceiptID == "" {
		t.Fatal("release failure lost unmatched return fence", result, err)
	}
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.ReturnedMinor != 5 || f.RefundedMinor != 0 || f.PendingOperationID != "" {
		t.Fatal("failed release altered original money facts", f, err)
	}
	before := len(provider.dispatched)
	provider.failKind = ""
	if result, err := svc.Execute(ctx, serviceCommand(src, "REFUND", "later-twenty", 20)); err != nil || result.State != "RECONCILIATION_REQUIRED" || len(provider.dispatched) != before {
		t.Fatal("later refund escaped failed release fence", result, err)
	}
}

func TestServiceSmallerRefundAfterReturnedSourceDenialPersistsFence(t *testing.T) {
	svc, store, funds, src, provider := servicePurchaseFixture(t)
	ctx := context.Background()
	src.original.AmountMinor = 100
	provider.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Execute(ctx, serviceCommand(src, "SETTLE", "accepted", 100)); err != nil {
		t.Fatal(err)
	}
	src.denyRefundAfterReturn = true
	_, _ = svc.Execute(ctx, serviceCommand(src, "REFUND", "source-denied-fifty", 50))
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.ReturnedMinor != 5 || f.RefundedMinor != 0 {
		t.Fatal("return-before-denial was not exercised", f, err)
	}
	src.denyRefundAfterReturn = false
	before := len(provider.dispatched)
	result, err := svc.Execute(ctx, serviceCommand(src, "REFUND", "later-twenty", 20))
	if err != nil || result.State != "RECONCILIATION_REQUIRED" || len(provider.dispatched) != before {
		t.Fatalf("negative commission adjustment dispatched refund: %+v %v", result, err)
	}
	order, err := store.ReadServicePurchase(ctx, src.original.OrderID)
	if err != nil || order.State != "RECONCILIATION_REQUIRED" {
		t.Fatal("guard did not persist original fence", order.State, err)
	}
}
