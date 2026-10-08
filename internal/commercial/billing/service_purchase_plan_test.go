package billing

import (
	"task-processor/internal/ledger/money"
	"testing"
)

func TestServicePlanCumulativeRefundAndZeroCommission(t *testing.T) {
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, OrderID: "original-order"}, ShareOperationID: "original-share", ShareProviderRequestID: "original-share-request"}
	c := ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, ID: "approval", Kind: "REFUND", AmountMinor: 2, SourceProofID: "both-parties-platform-approved"}
	f := money.ServiceFundsView{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, GrossMinor: 101, PlatformMinor: 10, ProviderMinor: 91, SharedMinor: 10, ReleasedMinor: 91}
	op, err := nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceReturn || op.Reservation.AmountMinor != 1 {
		t.Fatalf("partial refund must return 1 commission first: %+v %v", op, err)
	}
	f.ReturnedMinor = 1
	op, err = nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceRefund || op.Reservation.AmountMinor != 2 {
		t.Fatalf("after return must refund original 2: %+v %v", op, err)
	}
	c.Kind = "SETTLE"
	f = money.ServiceFundsView{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, GrossMinor: 5, ProviderMinor: 5}
	op, err = nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceShare || op.Reservation.AmountMinor != 0 {
		t.Fatalf("zero commission must have local proof: %+v %v", op, err)
	}
}
func TestServicePlanNeverFinishesBeforeCommissionAndKeepsReconciliation(t *testing.T) {
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, OrderID: "original-order"}}
	c := ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, ID: "accepted", Kind: "SETTLE", SourceProofID: "customer-acceptance"}
	f := money.ServiceFundsView{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, GrossMinor: 100, PlatformMinor: 10, ProviderMinor: 90}
	op, err := nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceShare {
		t.Fatalf("cannot finish before share: %+v %v", op, err)
	}
	f.ReconciliationReason = "external-chargeback"
	if _, err = nextServiceOperation(o, c, f); err != ErrReconciliationRequired {
		t.Fatalf("must preserve reconciliation: %v", err)
	}
}
func TestServicePartialShareRefundUsesApprovedRefundRelease(t *testing.T) {
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, OrderID: "original-order"}, ShareOperationID: "share", ShareProviderRequestID: "share-request"}
	c := ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, ID: "refund-approved", Kind: "REFUND", AmountMinor: 2, SourceProofID: "both-parties-platform-approved"}
	f := money.ServiceFundsView{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, GrossMinor: 101, PlatformMinor: 10, ProviderMinor: 91, SharedMinor: 10, ReturnedMinor: 1}
	op, err := nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceRefundRelease || op.Reservation.AmountMinor != 91 {
		t.Fatalf("partial share needs distinct approved refund release: %+v %v", op, err)
	}
	f.ReleasedMinor = 91
	op, err = nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceRefund {
		t.Fatalf("release wasn't original refund %+v %v", op, err)
	}
}

func TestServiceRefundPlanUsesOriginalAllocationSnapshot(t *testing.T) {
	p := money.ServiceAllocationPolicy{CommissionBPS: 2000, Basis: money.ServiceAllocationCumulativeNetFloorV1}
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{OrderID: "original", Allocation: p}, ShareOperationID: "share", ShareProviderRequestID: "original-share"}
	c := ServicePurchaseCommand{ID: "refund", Kind: "REFUND", AmountMinor: 2, SourceProofID: "approved", Allocation: p}
	f := money.ServiceFundsView{Allocation: p, GrossMinor: 101, PlatformMinor: 20, ProviderMinor: 81, SharedMinor: 20, ReleasedMinor: 81}
	op, err := nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.Kind != money.ServiceReturn || op.Reservation.AmountMinor != 1 {
		t.Fatalf("wrong frozen-rate commission return: %+v %v", op, err)
	}
	c.Allocation.CommissionBPS = 1000
	if _, err := nextServiceOperation(o, c, f); err != ErrConflict {
		t.Fatalf("changed refund rate accepted: %v", err)
	}
}

func TestServiceRefundPlanFencesUnmatchedCommissionReturn(t *testing.T) {
	p := money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{OrderID: "original", Allocation: p}, ShareOperationID: "share", ShareProviderRequestID: "original-share"}
	c := ServicePurchaseCommand{ID: "smaller-refund", Kind: "REFUND", AmountMinor: 20, SourceProofID: "approved", Allocation: p}
	f := money.ServiceFundsView{Allocation: p, GrossMinor: 100, PlatformMinor: 10, ProviderMinor: 90, SharedMinor: 10, ReturnedMinor: 5, ReleasedMinor: 90}
	if op, err := nextServiceOperation(o, c, f); err != ErrReconciliationRequired || op != nil {
		t.Fatalf("smaller refund misattributes unmatched returned commission: %+v %v", op, err)
	}
	// An original pre-settlement refund has no commission share to return.
	f.SharedMinor, f.ReturnedMinor, f.ReleasedMinor = 0, 0, 0
	if op, err := nextServiceOperation(o, c, f); err != nil || op == nil || op.Reservation.Kind != money.ServiceRefund {
		t.Fatalf("unsettled refund blocked: %+v %v", op, err)
	}
}
