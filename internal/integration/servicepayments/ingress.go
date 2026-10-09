package servicepayments

import (
	"context"
	"net/http"
	b "task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/ledger/money"
)

type NotificationVerifier interface {
	VerifyServiceNotification(*http.Request) (b.ServicePaymentObservation, error)
}
type NotificationInbox interface {
	ReadServicePurchaseByTrade(context.Context, string) (b.ServicePurchaseOrder, error)
	RecordServicePaymentObservation(context.Context, b.ServicePurchaseOrder, b.ServicePaymentObservation) error
}
type NotificationWake interface {
	WakeOriginalServicePurchase(context.Context, string) error
}
type NotificationIngress struct {
	Verifier NotificationVerifier
	Inbox    NotificationInbox
	Wake     NotificationWake
}

func (i NotificationIngress) AcceptNotification(req *http.Request) error {
	if req == nil || i.Verifier == nil || i.Inbox == nil || i.Wake == nil {
		return e.ErrUnavailable
	}
	p, err := i.Verifier.VerifyServiceNotification(req)
	if err != nil {
		return err
	}
	original, err := i.Inbox.ReadServicePurchaseByTrade(req.Context(), p.TradeNo)
	if err != nil {
		return err
	}
	if !p.Matches(original) {
		return b.ErrConflict
	}
	if (p.State == "PAID" || p.State == "PAID_REFUND_UNKNOWN") && original.Source.Allocation.Basis == money.ServiceAllocationCumulativeNetFloorV1 {
		// The signed notification includes channel facts before the original policy
		// is known. V1 accepts cash only and keeps its immutable original structure.
		if p.ChannelAmounts == nil || p.ChannelAmounts.Validate(original.Source.AmountMinor) != nil || p.ChannelAmounts.PayerMinor != original.Source.AmountMinor || len(p.ChannelAmounts.Vouchers) != 0 {
			return b.ErrConflict
		}
		p.ChannelAmounts = nil
	}
	if err := i.Inbox.RecordServicePaymentObservation(req.Context(), original, p); err != nil {
		return err
	}
	// ACK requires the durable verified inbox and an original recovery trigger.
	// A lost response repeats these same immutable facts, never a new purchase.
	return i.Wake.WakeOriginalServicePurchase(req.Context(), original.Source.OrderID)
}
