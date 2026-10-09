package money

import (
	"context"
	"strconv"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func voucherPayment(kind string, payer, voucher int64) m.ServicePaymentInput {
	in := servicePayment()
	in.Payment.GrossAmountMinor = payer + voucher
	in.Allocation.Basis = m.ServiceAllocationChannelNetFloorV2
	in.PolicyVersion = "ecoservices-channel-net-v2"
	in.ChannelAmounts = &m.ServicePaymentAmounts{PayerMinor: payer, Vouchers: []m.ServiceVoucher{{ID: "original-voucher", FundingType: kind, AmountMinor: voucher}}}
	return in
}

func TestSealedVoucherRefundPlanRejectsOutOfOrderMutationAndCumulativeOverRefund(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := voucherPayment("NOCASH", 80, 20)
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	run := func(op m.ServiceOperation, a *m.ServiceRefundAmounts) error {
		if _, err := r.PrepareServiceOperation(ctx, op); err != nil {
			return err
		}
		if op.AmountMinor > 0 {
			if err := r.AdmitServiceOperation(ctx, op); err != nil {
				return err
			}
		}
		_, err := r.AcceptServiceEffect(ctx, m.ServiceEffect{Operation: op, RefundAmounts: a, ProviderReference: "verified-" + op.OperationID, OccurredAt: time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)})
		return err
	}
	makePlan := func(command string, nominal int64) m.ServiceRefundPlan {
		f, err := r.ReadServiceFunds(ctx, in.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		p, err := m.NewServiceRefundPlan(f, command, "original-approval", command+"-pre", command+"-refund", command+"-post", command+"-release", "", nominal)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	op := func(p m.ServiceRefundPlan, phase string) m.ServiceOperation {
		kind, id, amount := m.ServiceReturn, p.PreOperationID, p.PreReturnMinor
		if phase == m.ServiceRefundPhase {
			kind, id, amount = m.ServiceRefund, p.RefundOperationID, p.NominalMinor
		}
		if phase == m.ServiceReturnPost {
			id = p.PostOperationID
			amount = 0
		}
		return m.ServiceOperation{RefundPlan: &p, RefundPhase: phase, OrderID: p.OrderID, OperationID: id, Kind: kind, AmountMinor: amount, SourceProofID: p.SourceProofID}
	}
	p := makePlan("one", 20)
	if _, err := r.PrepareServiceOperation(ctx, op(p, m.ServiceRefundPhase)); err == nil {
		t.Fatal("refund before original PRE admitted")
	}
	if err := run(op(p, m.ServiceReturnPre), nil); err != nil {
		t.Fatal(err)
	}
	changed := p
	changed.NominalMinor = 21
	if _, err := r.PrepareServiceOperation(ctx, op(changed, m.ServiceRefundPhase)); err == nil {
		t.Fatal("changed sealed original plan admitted")
	}
	a := m.ServiceRefundAmounts{DiscountMinor: 20, Vouchers: []m.ServiceVoucherRefund{{ID: "original-voucher", FundingType: "NOCASH", RefundType: "DISCOUNT", OriginalAmountMinor: 20, RefundMinor: 20}}}
	if err := run(op(p, m.ServiceRefundPhase), &a); err != nil {
		t.Fatal(err)
	}
	if err := run(op(p, m.ServiceReturnPost), nil); err != nil {
		t.Fatal(err)
	}
	p = makePlan("two", 20)
	if err := run(op(p, m.ServiceReturnPre), nil); err != nil {
		t.Fatal(err)
	}
	if err := run(op(p, m.ServiceRefundPhase), &a); err == nil {
		t.Fatal("same voucher refunded beyond original total")
	}
	f, _ := r.ReadServiceFunds(ctx, in.OrderID)
	if f.RefundedMinor != 20 || f.SettlementRefundedMinor != 0 || f.VoucherRefundedMinor["original-voucher"] != 20 {
		t.Fatalf("invalid actual refund mutated original funds %+v", f)
	}
}
func TestServicePaymentUsesActualSettlementIncome(t *testing.T) {
	for _, tc := range []struct {
		kind                                   string
		payer, voucher, settlement, commission int64
	}{{"CASH", 8000, 2000, 10000, 1000}, {"NOCASH", 8000, 2000, 8000, 800}, {"NOCASH", 0, 10000, 0, 0}} {
		t.Run(tc.kind+strconv.FormatInt(tc.payer, 10), func(t *testing.T) {
			r := newMoneyRepository(t)
			in := voucherPayment(tc.kind, tc.payer, tc.voucher)
			if _, err := r.AcceptServicePayment(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			f, err := r.ReadServiceFunds(context.Background(), in.OrderID)
			if err != nil || f.PlatformMinor != tc.commission || f.ProviderMinor != tc.settlement-tc.commission || f.ExpectedUnsplitMinor() != tc.settlement || f.ChannelAmounts == nil {
				t.Fatalf("wrong actual settlement: %+v %v", f, err)
			}
		})
	}
}
func TestServiceVoucherRefundCannotInventCashFact(t *testing.T) {
	r := newMoneyRepository(t)
	in := voucherPayment("NOCASH", 80, 20)
	ctx := context.Background()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	op := m.ServiceOperation{OrderID: in.OrderID, OperationID: "refund-without-components", Kind: m.ServiceRefund, SourceProofID: "approved", AmountMinor: 10}
	if _, err := r.PrepareServiceOperation(ctx, op); err == nil {
		t.Fatal("V2 refund without sealed original component plan was admitted")
	}
}
