package billing

import (
	"task-processor/internal/ledger/money"
	"testing"
)

func TestVoucherRefundStartsWithProvableMinimumReturn(t *testing.T) {
	policy := money.ServiceAllocationPolicy{Basis: money.ServiceAllocationChannelNetFloorV2, CommissionBPS: 1000}
	o := ServicePurchaseOrder{Source: ServicePurchaseCommand{OrderID: "order", Allocation: policy}, ShareOperationID: "share", ShareProviderRequestID: "original-share"}
	c := ServicePurchaseCommand{ID: "refund-command", OrderID: "order", SourceProofID: "approval", Kind: "REFUND", AmountMinor: 20, Allocation: policy}
	f := money.ServiceFundsView{OrderID: "order", PaymentID: "payment", Allocation: policy, GrossMinor: 100, SettlementMinor: 80, PlatformMinor: 8, ProviderMinor: 72, SharedMinor: 8, ReleasedMinor: 72, ChannelAmounts: &money.ServicePaymentAmounts{PayerMinor: 80, Vouchers: []money.ServiceVoucher{{ID: "coupon", FundingType: "NOCASH", AmountMinor: 20}}}}
	op, err := nextServiceOperation(o, c, f)
	if err != nil || op == nil || op.Reservation.RefundPhase != money.ServiceReturnPre || op.Reservation.AmountMinor != 0 || op.Reservation.RefundPlan == nil {
		t.Fatalf("must not guess refund components: %+v %v", op, err)
	}
}
