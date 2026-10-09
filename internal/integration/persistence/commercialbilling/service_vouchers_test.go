package commercialbilling

import (
	"context"
	"errors"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

type voucherProvider struct {
	*serviceProviderFixture
	payment                            money.ServicePaymentAmounts
	refunds                            map[string]money.ServiceRefundAmounts
	phases                             []string
	losePhase, failPhase, waitingPhase string
}

func (p *voucherProvider) QueryServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	v, err := p.serviceProviderFixture.QueryServicePayment(ctx, o)
	if v.State == "PAID" {
		a := p.payment.Normalize()
		v.ChannelAmounts = &a
	}
	return v, err
}
func (p *voucherProvider) QueryServiceUnsplit(_ context.Context, o billing.ServicePurchaseOrder) (billing.ServiceUnsplitObservation, error) {
	amount := p.payment.SettlementMinor()
	for _, e := range p.effects {
		if e.State == "SUCCESS" {
			switch e.Kind {
			case money.ServiceShare, money.ServiceFinish, money.ServiceRefundRelease:
				amount -= e.AmountMinor
			case money.ServiceRefund:
				amount -= e.RefundAmounts.SettlementMinor()
			}
		}
	}
	if amount < 0 {
		amount = 0
	}
	return billing.ServiceUnsplitObservation{ProfileVersion: o.Profile.Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: o.Payment.TransactionID, UnsplitMinor: amount, ProofID: "channel-unsplit", VerificationVersion: "signed-fixture", OccurredAt: time.Now().UTC()}, nil
}
func (p *voucherProvider) DispatchServiceOperation(ctx context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	v, err := p.serviceProviderFixture.DispatchServiceOperation(ctx, o, op)
	if err != nil {
		return v, err
	}
	p.phases = append(p.phases, op.Reservation.RefundPhase)
	if v.Kind == money.ServiceRefund {
		a, ok := p.refunds[op.CommandID]
		if !ok {
			return v, errors.New("missing actual channel refund fixture")
		}
		v.RefundAmounts = &a
	}
	if op.Reservation.RefundPhase == p.failPhase && p.failPhase != "" {
		v.State = "FAILED"
		v.Reason = "original-return-failed"
		v.RefundAmounts = nil
	}
	if op.Reservation.RefundPhase == p.waitingPhase && p.waitingPhase != "" {
		v.State = "WAITING_FUNDS"
		v.Reason = "original-balance-insufficient"
		v.RefundAmounts = nil
	}
	p.effects[op.ProviderRequestID] = v
	if op.Reservation.RefundPhase == p.losePhase && p.losePhase != "" {
		p.losePhase = ""
		return billing.ServiceOperationObservation{}, errors.New("lost original phase acknowledgement")
	}
	return v, nil
}
func voucherService(t *testing.T, amounts money.ServicePaymentAmounts) (*billing.ServicePurchases, *Repository, money.ServiceFundsStore, *serviceSourceFixture, *voucherProvider) {
	t.Helper()
	_, r, f, src, base := servicePurchaseFixture(t)
	src.original.AmountMinor = amounts.PayerMinor
	for _, v := range amounts.Vouchers {
		src.original.AmountMinor += v.AmountMinor
	}
	src.original.Allocation.Basis = money.ServiceAllocationChannelNetFloorV2
	src.original.PolicyVersion = "ecoservices-v2-channel-net-10-platform-fee-manual-expiry"
	base.paid = true
	p := &voucherProvider{serviceProviderFixture: base, payment: amounts, refunds: map[string]money.ServiceRefundAmounts{}}
	s, err := billing.NewServicePurchases(r, f, p, src, serviceProtectionFixture{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Execute(context.Background(), src.original); err != nil {
		t.Fatal(err)
	}
	return s, r, f, src, p
}
func runVoucherCommand(t *testing.T, s *billing.ServicePurchases, c billing.ServicePurchaseCommand) billing.ServicePurchaseResult {
	t.Helper()
	var r billing.ServicePurchaseResult
	var err error
	for i := 0; i < 6; i++ {
		r, err = s.Execute(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		if r.State == "SETTLED" || r.State == "REFUNDED" {
			return r
		}
	}
	t.Fatalf("original command did not finish %+v", r)
	return r
}
func TestVoucherRefundUsesActualComponentsAndCumulativeFloor(t *testing.T) {
	s, r, f, src, p := voucherService(t, money.ServicePaymentAmounts{PayerMinor: 60, Vouchers: []money.ServiceVoucher{{ID: "cash", FundingType: "CASH", AmountMinor: 20}, {ID: "discount", FundingType: "NOCASH", AmountMinor: 20}}})
	runVoucherCommand(t, s, serviceCommand(src, "SETTLE", "accepted", 100))
	refunds := []struct {
		id                                       string
		nominal, payer, cash, discount, netShare int64
	}{{"one", 20, 10, 5, 5, 6}, {"two", 30, 20, 5, 5, 4}, {"three", 50, 30, 10, 10, 0}}
	for _, x := range refunds {
		p.refunds[x.id] = money.ServiceRefundAmounts{PayerMinor: x.payer, DiscountMinor: x.cash + x.discount, Vouchers: []money.ServiceVoucherRefund{{ID: "cash", FundingType: "CASH", RefundType: "COUPON", OriginalAmountMinor: 20, RefundMinor: x.cash}, {ID: "discount", FundingType: "NOCASH", RefundType: "DISCOUNT", OriginalAmountMinor: 20, RefundMinor: x.discount}}}
		result := runVoucherCommand(t, s, serviceCommand(src, "REFUND", x.id, x.nominal))
		funds, err := f.ReadServiceFunds(context.Background(), src.original.OrderID)
		if err != nil || funds.SharedMinor-funds.ReturnedMinor != x.netShare || funds.PlatformMinor != x.netShare || funds.PendingRefundCommandID != "" {
			t.Fatalf("wrong cumulative floor %+v %+v %v", result, funds, err)
		}
		view, err := s.ReadFinancialFacts(context.Background(), src.original.OrderID)
		if err != nil || view.ChannelAmounts == nil || view.ChannelAmounts.PayerMinor != 60 || view.ChannelAmounts.SettlementMinor != 80 || view.ChannelAmounts.SettlementRefundedMinor != funds.SettlementRefundedMinor {
			t.Fatalf("original typed financial projection lost channel facts %+v %v", view, err)
		}
	}
	o, _ := r.ReadServicePurchase(context.Background(), src.original.OrderID)
	if !o.CompletedCommands["three"].FullRefund {
		t.Fatal("nominal full refund lost")
	}
	for id, n := range p.dispatched {
		if n != 1 {
			t.Fatalf("replayed %s %d", id, n)
		}
	}
}
func TestVoucherRefundBeforeShareHasOnlyOriginalRefundDispatch(t *testing.T) {
	for _, kind := range []string{"REFUND", "CANCEL"} {
		t.Run(kind, func(t *testing.T) {
			s, _, f, src, p := voucherService(t, money.ServicePaymentAmounts{PayerMinor: 0, Vouchers: []money.ServiceVoucher{{ID: "discount", FundingType: "NOCASH", AmountMinor: 100}}})
			p.refunds["original-refund"] = money.ServiceRefundAmounts{DiscountMinor: 100, Vouchers: []money.ServiceVoucherRefund{{ID: "discount", FundingType: "NOCASH", RefundType: "COUPON", OriginalAmountMinor: 100, RefundMinor: 100}}}
			runVoucherCommand(t, s, serviceCommand(src, kind, "original-refund", 100))
			funds, _ := f.ReadServiceFunds(context.Background(), src.original.OrderID)
			if len(p.dispatched) != 1 || funds.SettlementRefundedMinor != 0 || funds.RefundedMinor != 100 || funds.SharedMinor != 0 || funds.PendingRefundCommandID != "" {
				t.Fatalf("unsplit zero-income refund invented share/return %+v %+v", funds, p.dispatched)
			}
		})
	}
}
func TestVoucherRefundPostReturnLossWaitAndFailureKeepActualRefundAndHold(t *testing.T) {
	for _, mode := range []string{"lost", "waiting", "failed"} {
		t.Run(mode, func(t *testing.T) {
			s, r, f, src, p := voucherService(t, money.ServicePaymentAmounts{PayerMinor: 80, Vouchers: []money.ServiceVoucher{{ID: "discount", FundingType: "NOCASH", AmountMinor: 20}}})
			runVoucherCommand(t, s, serviceCommand(src, "SETTLE", "accepted", 100))
			p.refunds["original-refund"] = money.ServiceRefundAmounts{PayerMinor: 20}
			switch mode {
			case "lost":
				p.losePhase = money.ServiceReturnPost
			case "waiting":
				p.waitingPhase = money.ServiceReturnPost
			case "failed":
				p.failPhase = money.ServiceReturnPost
			}
			c := serviceCommand(src, "REFUND", "original-refund", 20)
			for i := 0; i < 4; i++ {
				_, err := s.Execute(context.Background(), c)
				if err != nil {
					break
				}
				o, _ := r.ReadServicePurchase(context.Background(), c.OrderID)
				if o.Operation != nil && o.Operation.Reservation.RefundPhase == money.ServiceReturnPost {
					break
				}
			}
			funds, _ := f.ReadServiceFunds(context.Background(), c.OrderID)
			o, _ := r.ReadServicePurchase(context.Background(), c.OrderID)
			if funds.RefundedMinor != 20 || funds.SettlementRefundedMinor != 20 || funds.PendingRefundCommandID != c.ID || o.CompletedCommands[c.ID].State == "REFUNDED" {
				t.Fatalf("refund success/hold lost %+v %+v", funds, o)
			}
			if _, err := f.PrepareServiceOperation(context.Background(), money.ServiceOperation{OrderID: c.OrderID, OperationID: "unrelated-share", Kind: money.ServiceShare, AmountMinor: 0, SourceProofID: "unrelated"}); err == nil {
				t.Fatal("new funds dispatched before actual POST")
			}
			if mode == "lost" {
				runVoucherCommand(t, s, c)
			} else {
				_, _ = s.Execute(context.Background(), c)
				o, _ = r.ReadServicePurchase(context.Background(), c.OrderID)
				if o.ActiveCommand == nil || o.Operation == nil || o.CompletedCommands[c.ID].State == "REFUNDED" {
					t.Fatal("waiting/failed POST released original hold")
				}
			}
			for id, n := range p.dispatched {
				if n != 1 {
					t.Fatalf("replayed %s %d", id, n)
				}
			}
		})
	}
}
