package ecoservices

import (
	"github.com/google/uuid"
	"time"
)

func TransitionRequest(r *Request, c Command, now time.Time) (*FinancialCommand, error) {
	if r == nil || c.Version != r.Version {
		return nil, ErrConflict
	}
	buyer := !c.Scope.Platform && c.Scope.OrganizationID == r.BuyerOrganizationID
	provider := !c.Scope.Platform && c.Scope.OrganizationID == r.ProviderOrganizationID
	switch c.Kind {
	case "quote", "start", "deliver":
		if !provider {
			return nil, ErrForbidden
		}
	case "confirm_quote", "accept", "reject", "cancel":
		if !buyer {
			return nil, ErrForbidden
		}
	case "refund_propose", "refund_confirm":
		if !buyer && !provider {
			return nil, ErrForbidden
		}
	case "refund_review", "refund_review_reject":
		if !c.Scope.Platform {
			return nil, ErrForbidden
		}
	default:
		return nil, ErrInvalid
	}
	var financial *FinancialCommand
	makeCommand := func(kind string, amount int64) *FinancialCommand {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ecoservices:"+r.ID+":"+kind+":"+c.Key)).String()
		quote := Quote{}
		if r.Quote != nil {
			quote = *r.Quote
		}
		return &FinancialCommand{ID: id, RequestID: r.ID, OrderID: r.OrderID, Kind: kind, SourceProofID: id, ActorID: c.Scope.ActorID, BuyerOrganizationID: r.BuyerOrganizationID, ProviderOrganizationID: r.ProviderOrganizationID, Quote: quote, AmountMinor: amount, PolicyVersion: quote.PolicyVersion, State: "PENDING"}
	}
	switch c.Kind {
	case "quote":
		if r.State != "REQUESTED" && r.State != "QUOTED" {
			return nil, ErrConflict
		}
		if c.Quote == nil || c.Quote.AmountMinor <= 0 || !validText(c.Quote.Scope, 10000) || !validText(c.Quote.AcceptanceCriteria, 5000) || c.Quote.DeliveryDays < 1 {
			return nil, ErrInvalid
		}
		q := *c.Quote
		q.CommissionBPS = 1000
		q.AllocationBasis = "CUMULATIVE_NET_FLOOR_V1"
		q.PolicyVersion = PolicyVersion
		q.Version = 1
		if r.Quote != nil {
			q.Version = r.Quote.Version + 1
		}
		r.Quote = &q
		r.State = "QUOTED"
	case "confirm_quote":
		if r.State != "QUOTED" || r.Quote == nil || c.Quote == nil || c.Quote.Version != r.Quote.Version || c.PolicyAccepted != r.Quote.PolicyVersion || r.Quote.CommissionBPS != 1000 || r.Quote.AllocationBasis != "CUMULATIVE_NET_FLOOR_V1" {
			return nil, ErrConflict
		}
		r.OrderID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("ecoservices-order:"+r.ID)).String()
		r.State = "ORDER_PENDING"
		r.FinancialState = "AWAITING_PAYMENT"
		financial = makeCommand("CREATE_PURCHASE", r.Quote.AmountMinor)
	case "start":
		if r.State != "PAID_READY" || r.PaymentReceiptID == "" || r.FinancialFence {
			return nil, ErrConflict
		}
		r.State = "SERVICING"
	case "deliver":
		if r.State != "SERVICING" || r.FinancialFence {
			return nil, ErrConflict
		}
		if c.Delivery == nil || !validText(c.Delivery.Content, 10000) || len(c.Delivery.FileIDs) > 10 {
			return nil, ErrInvalid
		}
		d := *c.Delivery
		d.Rejection = nil
		d.Version = 1
		if r.Delivery != nil {
			d.Version = r.Delivery.Version + 1
		}
		d.SubmittedAt = now
		d.FileIDs = append([]string(nil), d.FileIDs...)
		r.Delivery = &d
		r.State = "AWAITING_ACCEPTANCE"
	case "accept", "reject":
		if r.State != "AWAITING_ACCEPTANCE" || r.Delivery == nil || c.DeliveryVersion != r.Delivery.Version || r.AcceptanceID != "" || r.FinancialFence {
			return nil, ErrConflict
		}
		if c.Kind == "reject" {
			if !validText(c.Reason, 2000) {
				return nil, ErrInvalid
			}
			r.State = "SERVICING"
			r.Delivery.Rejection = &DeliveryRejection{DeliveryVersion: r.Delivery.Version, Reason: c.Reason, ActorID: c.Scope.ActorID, RejectedAt: now}
			break
		}
		financial = makeCommand("SETTLE", r.Quote.AmountMinor)
		r.AcceptanceID = financial.SourceProofID
		r.AcceptedDeliveryVersion = r.Delivery.Version
		r.State = "ACCEPTED"
		r.FinancialState = "SETTLEMENT_PENDING"
	case "cancel":
		switch r.State {
		case "REQUESTED", "QUOTED":
			r.State = "CANCELLED"
		case "ORDER_PENDING", "PAID_READY":
			if r.FinancialFence {
				return nil, ErrConflict
			}
			r.State = "CANCEL_REQUESTED"
			r.FinancialFence = true
			r.FinancialState = "CANCELLATION_PENDING"
			financial = makeCommand("CANCEL", r.Quote.AmountMinor)
		default:
			return nil, ErrConflict
		}
	case "refund_propose":
		if r.State != "SERVICING" && r.State != "AWAITING_ACCEPTANCE" && r.State != "ACCEPTED" {
			return nil, ErrConflict
		}
		if r.Refund != nil && r.Refund.State == "APPROVED" {
			return nil, ErrConflict
		}
		if r.Quote == nil || c.RefundAmountMinor <= 0 || c.RefundAmountMinor > r.Quote.AmountMinor || !validText(c.Reason, 2000) {
			return nil, ErrInvalid
		}
		if c.RefundableAmount == nil {
			return nil, ErrUnavailable
		}
		if c.RefundAmountMinor > *c.RefundableAmount {
			return nil, ErrInvalid
		}
		version := int64(1)
		if r.Refund != nil {
			version = r.Refund.Version + 1
		}
		r.Refund = &RefundAgreement{Version: version, AmountMinor: c.RefundAmountMinor, Reason: c.Reason, BuyerConfirmed: buyer, ProviderConfirmed: provider, State: "NEGOTIATING"}
		r.FinancialFence = true
	case "refund_confirm":
		if r.Refund == nil || r.Refund.State != "NEGOTIATING" || c.RefundVersion != r.Refund.Version {
			return nil, ErrConflict
		}
		if buyer {
			r.Refund.BuyerConfirmed = true
		} else {
			r.Refund.ProviderConfirmed = true
		}
	case "refund_review", "refund_review_reject":
		if r.Refund == nil || r.Refund.State != "NEGOTIATING" || c.RefundVersion != r.Refund.Version || !validText(c.Reason, 2000) {
			return nil, ErrConflict
		}
		if c.Kind == "refund_review_reject" {
			r.Refund.State = "REJECTED"
			r.Refund.Review = &RefundReview{Reason: c.Reason, ActorID: c.Scope.ActorID, ReviewedAt: now}
			// Closing this dispute cannot clear an independent channel money hold.
			r.FinancialFence = r.FinancialState == "RECONCILIATION_REQUIRED"
			break
		}
		if !r.Refund.BuyerConfirmed || !r.Refund.ProviderConfirmed {
			return nil, ErrConflict
		}
		if c.RefundableAmount == nil {
			return nil, ErrUnavailable
		}
		if r.Refund.AmountMinor > *c.RefundableAmount {
			return nil, ErrInvalid
		}
		r.Refund.State = "APPROVED"
		r.Refund.Review = &RefundReview{Reason: c.Reason, ActorID: c.Scope.ActorID, ReviewedAt: now}
		r.FinancialFence = true
		r.FinancialState = "REFUND_PENDING"
		financial = makeCommand("REFUND", r.Refund.AmountMinor)
	}
	r.Version++
	r.UpdatedAt = now
	return financial, nil
}

// ApplyFinancialResult only projects canonical trading receipts; expiry facts
// are independent of fulfillment, especially customer acceptance.
func ApplyFinancialResult(r *Request, result FinancialResult, now time.Time) error {
	if r == nil || r.OrderID == "" || result.OrderID != r.OrderID {
		return ErrConflict
	}
	if result.Revision > 0 && result.Revision < r.FinancialRevision {
		return nil
	}
	// FinancialRevision is internal ordering metadata (json:"-"). A lease
	// claim can advance it without changing the user's request projection.
	beforeProjection := Fingerprint(*r)
	beforePaymentReceipt := r.PaymentReceiptID
	if result.Revision > 0 {
		r.FinancialRevision = result.Revision
	}
	if result.PaymentReceiptID != "" {
		if r.PaymentReceiptID != "" && r.PaymentReceiptID != result.PaymentReceiptID {
			return ErrConflict
		}
		r.PaymentReceiptID = result.PaymentReceiptID
		if r.State == "ORDER_PENDING" && result.State != "RECONCILIATION_REQUIRED" {
			r.State = "PAID_READY"
		}
	}
	r.FinancialState = result.State
	r.FinancialReason = result.Reason
	if result.State == "RECONCILIATION_REQUIRED" {
		r.FinancialFence = true
	}
	r.FundsExpireAt = result.FundsExpireAt
	if result.State == "CLOSED_UNPAID" && result.ReceiptID != "" && r.PaymentReceiptID == "" && (r.State == "ORDER_PENDING" || r.State == "CANCEL_REQUESTED") {
		r.State = "CANCELLED"
	}
	if result.State == "REFUNDED" {
		if r.State == "CANCEL_REQUESTED" || result.FullRefund {
			r.State = "CANCELLED"
		}
		if r.Refund != nil && r.Refund.State == "APPROVED" {
			r.Refund.State = "REFUNDED"
		}
		r.FinancialFence = false
	}
	if result.State == "CHANNEL_OPERATION_FAILED" && r.Refund != nil && r.Refund.State == "APPROVED" {
		r.Refund.State = "FAILED"
	}
	if beforeProjection != "" && Fingerprint(*r) == beforeProjection && r.PaymentReceiptID == beforePaymentReceipt {
		return nil
	}
	r.Version++
	r.UpdatedAt = now
	return nil
}
