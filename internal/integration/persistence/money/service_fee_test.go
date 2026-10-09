package money

import (
	"context"
	"errors"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestServiceChannelFeeOriginalFlowCannotChangeAllocationOrDoubleBook(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	payment := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, payment); err != nil {
		t.Fatal(err)
	}
	fee := m.ServiceChannelFee{Payment: payment, FlowID: "original-platform-fee-flow", BusinessID: payment.Binding.TradeID, ProofID: "signed-fees-bill-hash", AmountMinor: 3, OccurredAt: time.Now().UTC()}
	a, err := r.ObserveServiceChannelFee(ctx, fee)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ObserveServiceChannelFee(ctx, fee)
	if err != nil || a != b {
		t.Fatal("duplicate fee receipt changed")
	}
	funds, _ := r.ReadServiceFunds(ctx, payment.OrderID)
	if funds.ChannelFeeMinor != 3 || !funds.ChannelFeeObserved || funds.PlatformMinor != 10 || funds.ProviderMinor != 91 {
		t.Fatalf("fees changed commission/provider split: %+v", funds)
	}
	fee.AmountMinor = 4
	if _, err = r.ObserveServiceChannelFee(ctx, fee); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("same original flow changed amount: %v", err)
	}
	fee.FlowID = "return-fee"
	fee.AmountMinor = 2
	fee.Returned = true
	if _, err = r.ObserveServiceChannelFee(ctx, fee); err != nil {
		t.Fatal(err)
	}
	funds, _ = r.ReadServiceFunds(ctx, payment.OrderID)
	if funds.ChannelFeeMinor != 1 || funds.ProviderMinor != 91 {
		t.Fatal("fee return changed provider allocation")
	}
	fee.FlowID = "unproved-return"
	fee.AmountMinor = 2
	if _, err = r.ObserveServiceChannelFee(ctx, fee); !errors.Is(err, m.ErrConflict) {
		t.Fatal("unobserved fee refund created income")
	}
}
