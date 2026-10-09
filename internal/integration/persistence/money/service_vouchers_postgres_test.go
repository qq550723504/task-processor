//go:build integration

package money

import (
	"sync"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestServicePostgresVoucherPlansSerializeAndPreserveActualRefund(t *testing.T) {
	ctx, _, r, _ := newMoneyPostgresRuntime(t)
	in := voucherPayment("NOCASH", 80, 20)
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	f, err := r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	plans := make([]m.ServiceRefundPlan, 2)
	for i, id := range []string{"original-a", "original-b"} {
		plans[i], err = m.NewServiceRefundPlan(f, id, "approval", id+"-pre", id+"-refund", id+"-post", id+"-release", "", 20)
		if err != nil {
			t.Fatal(err)
		}
	}
	pre := func(p m.ServiceRefundPlan) m.ServiceOperation {
		return m.ServiceOperation{RefundPlan: &p, RefundPhase: m.ServiceReturnPre, OrderID: p.OrderID, OperationID: p.PreOperationID, Kind: m.ServiceReturn, AmountMinor: 0, SourceProofID: p.SourceProofID}
	}
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := range plans {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := r.PrepareServiceOperation(ctx, pre(plans[i])); err == nil {
				results <- i
			}
		}(i)
	}
	wg.Wait()
	close(results)
	if len(results) != 1 {
		t.Fatalf("competing original plans admitted %d", len(results))
	}
	p := plans[<-results]
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	if _, err := r.AcceptServiceEffect(ctx, m.ServiceEffect{Operation: pre(p), ProviderReference: "local-zero-return", OccurredAt: at}); err != nil {
		t.Fatal(err)
	}
	refund := m.ServiceOperation{RefundPlan: &p, RefundPhase: m.ServiceRefundPhase, OrderID: p.OrderID, OperationID: p.RefundOperationID, Kind: m.ServiceRefund, AmountMinor: 20, SourceProofID: p.SourceProofID}
	if _, err := r.PrepareServiceOperation(ctx, refund); err != nil {
		t.Fatal(err)
	}
	if err := r.AdmitServiceOperation(ctx, refund); err != nil {
		t.Fatal(err)
	}
	actual := m.ServiceRefundAmounts{PayerMinor: 16, DiscountMinor: 4, Vouchers: []m.ServiceVoucherRefund{{ID: "original-voucher", FundingType: "NOCASH", RefundType: "COUPON", OriginalAmountMinor: 20, RefundMinor: 4}}}
	receipt, err := r.AcceptServiceEffect(ctx, m.ServiceEffect{Operation: refund, RefundAmounts: &actual, ProviderReference: "original-verified-refund", OccurredAt: at})
	if err != nil {
		t.Fatal(err)
	}
	read, err := r.ReadServiceEffect(ctx, refund)
	if err != nil || read.RefundAmounts == nil || read.ResultFingerprint != receipt.ResultFingerprint || read.RefundAmounts.SettlementMinor() != 16 {
		t.Fatalf("lost response cannot read actual original refund %+v %v", read, err)
	}
	f, err = r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || f.RefundedMinor != 20 || f.SettlementRefundedMinor != 16 || f.PayerRefundedMinor != 16 || f.VoucherRefundedMinor["original-voucher"] != 4 || f.PendingRefundCommandID != p.CommandID {
		t.Fatalf("refund fact/POST hold lost %+v %v", f, err)
	}
	post := pre(p)
	post.RefundPhase = m.ServiceReturnPost
	post.OperationID = p.PostOperationID
	if _, err := r.PrepareServiceOperation(ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AcceptServiceEffect(ctx, m.ServiceEffect{Operation: post, ProviderReference: "local-zero-post", OccurredAt: at}); err != nil {
		t.Fatal(err)
	}
	f, err = r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || f.PendingRefundCommandID != "" || f.PlatformMinor != 6 || f.ProviderMinor != 58 {
		t.Fatalf("wrong remaining original income %+v %v", f, err)
	}
}
