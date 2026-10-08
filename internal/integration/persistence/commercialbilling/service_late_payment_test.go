package commercialbilling

import (
	"context"
	"testing"

	"task-processor/internal/ledger/money"
)

func TestServiceLatePaymentAfterClosedCancellationContinuesOriginalRefund(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirmed", true: "lost-refund-response"}[lost], func(t *testing.T) {
			svc, store, funds, src, p := servicePurchaseFixture(t)
			ctx := context.Background()
			if _, err := svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Checkout(ctx, src.original.BuyerOrganizationID, src.original.ActorID, src.original.OrderID); err != nil {
				t.Fatal(err)
			}
			src.cancel = true
			cancel := serviceCommand(src, "CANCEL", "original-cancel", 101)
			result, err := svc.Execute(ctx, cancel)
			if err != nil || result.State != "CLOSED_UNPAID" || result.PaymentReceiptID != "" {
				t.Fatalf("original unpaid cancellation failed: %+v %v", result, err)
			}
			original, _ := store.ReadServicePurchase(ctx, src.original.OrderID)
			p.paid = true
			payment, _ := p.QueryServicePayment(ctx, original)
			if err := store.RecordServicePaymentObservation(ctx, original, payment); err != nil {
				t.Fatal(err)
			}
			result, err = svc.Execute(ctx, src.original)
			if err != nil || result.State != "CANCELLATION_PENDING" || result.PaymentReceiptID == "" {
				t.Fatalf("late payment retained unpaid closure instead of original cancellation: %+v %v", result, err)
			}
			p.enabled = false // Closing new payments must retain original refunds.
			p.lostEffect = lost
			result, err = svc.Execute(ctx, cancel)
			if lost {
				if err == nil {
					t.Fatal("lost refund response did not interrupt")
				}
				result, err = svc.Execute(ctx, cancel)
			}
			if err != nil || result.State != "REFUNDED" || !result.FullRefund || result.ReceiptID == "" {
				t.Fatalf("original cancellation did not refund late payment: %+v %v", result, err)
			}
			for n := 0; n < 2; n++ {
				if err := store.RecordServicePaymentObservation(ctx, original, payment); err != nil {
					t.Fatal(err)
				}
				if _, err := svc.Execute(ctx, src.original); err != nil {
					t.Fatal(err)
				}
				if replay, err := svc.Execute(ctx, cancel); err != nil || replay.ReceiptID != result.ReceiptID {
					t.Fatalf("duplicate payment changed original refund: %+v %v", replay, err)
				}
			}
			current, _ := store.ReadServicePurchase(ctx, src.original.OrderID)
			view, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
			if err != nil || view.RefundedMinor != 101 || view.SharedMinor != 0 || view.ReleasedMinor != 0 || !current.CancelRequested || current.TradeNo != original.TradeNo || current.Source.Fingerprint() != original.Source.Fingerprint() {
				t.Fatalf("late refund changed original funds/identity: %+v %v", view, err)
			}
			if len(p.dispatched) != 1 || p.creates != 1 || p.closes != 1 {
				t.Fatalf("late payment created extra effects: %+v creates=%d closes=%d", p.dispatched, p.creates, p.closes)
			}
			for id, count := range p.dispatched {
				if count != 1 || p.effects[id].Kind != money.ServiceRefund || current.Operations["service-operation:"+id].CommandID != cancel.ID {
					t.Fatalf("refund did not retain original cancellation identity: %s=%d %+v", id, count, current.Operations)
				}
			}
			observations, _ := store.ServicePaymentObservations(ctx, src.original.OrderID)
			closed := false
			for _, observation := range observations {
				closed = closed || observation.State == "CLOSED"
			}
			if !closed {
				t.Fatal("original closed-unpaid proof was removed")
			}
		})
	}
}
