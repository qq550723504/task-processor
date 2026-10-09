package commercialbilling

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
)

type naturalClosureProvider struct {
	*serviceProviderFixture
	queries    int
	queryError bool
}

func (p *naturalClosureProvider) QueryServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	p.queries++
	if p.queryError {
		return billing.ServicePaymentObservation{}, errors.New("original query unavailable")
	}
	result, err := p.serviceProviderFixture.QueryServicePayment(ctx, o)
	if result.State == "UNPAID" {
		result.State, result.EventID = "CLOSED", "natural-closed:"+o.TradeNo
	}
	return result, err
}

type naturalClosureSaveFault struct {
	billing.ServicePurchaseStore
	failOnce bool
}

func (f *naturalClosureSaveFault) SaveServicePurchase(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePurchaseOrder, error) {
	if f.failOnce && o.State == "CLOSED_UNPAID" {
		f.failOnce = false
		return o, errors.New("closure projection save unavailable")
	}
	return f.ServicePurchaseStore.SaveServicePurchase(ctx, o)
}

func TestServiceNaturalClosureCompletesOriginalCreateAndRecoversStoredProof(t *testing.T) {
	for _, lostSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirmed", true: "lost-closure-save"}[lostSave], func(t *testing.T) {
			_, store, funds, src, base := servicePurchaseFixture(t)
			provider := &naturalClosureProvider{serviceProviderFixture: base}
			fault := &naturalClosureSaveFault{ServicePurchaseStore: store, failOnce: lostSave}
			svc, err := billing.NewServicePurchases(fault, funds, provider, src, serviceProtectionFixture{})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if _, err := svc.Checkout(ctx, src.original.BuyerOrganizationID, src.original.ActorID, src.original.OrderID); err != nil {
				t.Fatal(err)
			}
			result, err := svc.Execute(ctx, src.original)
			if lostSave {
				if err == nil {
					t.Fatal("closure save failure not exercised")
				}
				observations, _ := store.ServicePaymentObservations(ctx, src.original.OrderID)
				if len(observations) != 1 || observations[0].State != "CLOSED" {
					t.Fatalf("original close proof not durable: %+v", observations)
				}
				if _, err := svc.Checkout(ctx, src.original.BuyerOrganizationID, src.original.ActorID, src.original.OrderID); err == nil {
					t.Fatal("durable close inbox returned cached QR after closure save loss")
				}
				provider.queryError = true
				result, err = svc.Execute(ctx, src.original)
			}
			if err != nil || result.State != "CLOSED_UNPAID" || result.ReceiptID == "" || result.PaymentReceiptID != "" {
				t.Fatalf("verified close not terminal: %+v %v", result, err)
			}
			provider.queryError = true
			if _, err := svc.Checkout(ctx, src.original.BuyerOrganizationID, src.original.ActorID, src.original.OrderID); err == nil {
				t.Fatal("verified closed original returned cached QR before E projection")
			}
			for i := 0; i < 2; i++ {
				if replay, err := svc.Execute(ctx, src.original); err != nil || replay.ReceiptID != result.ReceiptID {
					t.Fatalf("original close replay queried again: %+v %v", replay, err)
				}
			}
			o, _ := store.ReadServicePurchase(ctx, src.original.OrderID)
			if provider.queries != 1 || o.CancelRequested || o.ActiveCommand != nil || o.Operation != nil || len(o.CompletedCommands) != 1 || len(base.dispatched) != 0 || base.closes != 0 {
				t.Fatalf("closure invented cancellation/effect or kept polling: %+v queries=%d effects=%+v", o, provider.queries, base.dispatched)
			}
			if _, err := funds.ReadServiceFunds(ctx, src.original.OrderID); !errors.Is(err, money.ErrNotFound) {
				t.Fatalf("unpaid closure created money: %v", err)
			}
		})
	}
}

func TestServiceLatePaymentAfterNaturalClosureKeepsOriginalPaymentForReconciliation(t *testing.T) {
	for _, lostSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "cached-close", true: "close-save-lost"}[lostSave], func(t *testing.T) {
			_, store, funds, src, base := servicePurchaseFixture(t)
			provider := &naturalClosureProvider{serviceProviderFixture: base}
			fault := &naturalClosureSaveFault{ServicePurchaseStore: store, failOnce: lostSave}
			svc, err := billing.NewServicePurchases(fault, funds, provider, src, serviceProtectionFixture{})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if _, err := svc.Checkout(ctx, src.original.BuyerOrganizationID, src.original.ActorID, src.original.OrderID); err != nil {
				t.Fatal(err)
			}
			closed, err := svc.Execute(ctx, src.original)
			if lostSave && err == nil {
				t.Fatal("closure save failure not exercised")
			}
			if !lostSave && (err != nil || closed.State != "CLOSED_UNPAID") {
				t.Fatalf("close: %+v %v", closed, err)
			}
			o, _ := store.ReadServicePurchase(ctx, src.original.OrderID)
			base.paid = true
			paid, err := provider.QueryServicePayment(ctx, o)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.RecordServicePaymentObservation(ctx, o, paid); err != nil {
				t.Fatal(err)
			}
			provider.queryError = true
			result, err := svc.Execute(ctx, src.original)
			if err != nil || result.State != "RECONCILIATION_REQUIRED" || result.Reason != "PAYMENT_AFTER_UNPAID_CLOSE" || result.PaymentReceiptID == "" {
				t.Fatalf("late payment hidden by close or became deliverable: %+v %v", result, err)
			}
			view, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
			current, _ := store.ReadServicePurchase(ctx, src.original.OrderID)
			receipt, receiptErr := funds.ReadServicePayment(ctx, current.MoneyInput())
			if err != nil || receiptErr != nil || receipt.ReceiptID != result.PaymentReceiptID || current.Payment == nil || current.Payment.TransactionID != paid.TransactionID || view.RefundedMinor != 0 || view.SharedMinor != 0 || view.PendingOperationID != "" || current.CancelRequested || current.TradeNo != o.TradeNo || len(base.dispatched) != 0 {
				t.Fatalf("natural closure invented refund or lost payment: %+v %v order=%+v", view, err, current)
			}
			if _, err := svc.Execute(ctx, serviceCommand(src, "SETTLE", "must-not-dispatch", 101)); err != nil {
				t.Fatal(err)
			}
			if len(base.dispatched) != 0 {
				t.Fatal("late payment reconciliation dispatched funds")
			}
		})
	}
}
