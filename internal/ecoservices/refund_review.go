package ecoservices

import "context"

// This internal contract carries the original review identity, not a caller's
// claimed funds balance. Only the original trading adapter supplies its proof.
type RefundReviewAdmission struct {
	Scope                                                               Scope
	Key, Kind, RequestID, OrderID, PaymentReceiptID, CommandFingerprint string
	BuyerOrganizationID, ProviderOrganizationID                         string
	RequestVersion, RefundVersion, QuoteVersion, AmountMinor            int64
}

func BuildRefundReviewAdmission(c Command, r Request) (RefundReviewAdmission, error) {
	if !c.Scope.Platform || c.Kind != "refund_review" {
		return RefundReviewAdmission{}, ErrForbidden
	}
	copyCommand := c
	copyCommand.Fingerprint = ""
	if c.Key == "" || c.ID != r.ID || c.Version != r.Version || c.Fingerprint != Fingerprint(copyCommand) || r.Quote == nil || r.Refund == nil || r.PaymentReceiptID == "" || r.Refund.State != "NEGOTIATING" || !r.Refund.BuyerConfirmed || !r.Refund.ProviderConfirmed || c.RefundVersion != r.Refund.Version || r.FinancialState == "RECONCILIATION_REQUIRED" {
		return RefundReviewAdmission{}, ErrConflict
	}
	return RefundReviewAdmission{Scope: c.Scope, Key: c.Key, Kind: c.Kind, RequestID: r.ID, OrderID: r.OrderID, PaymentReceiptID: r.PaymentReceiptID, CommandFingerprint: c.Fingerprint, BuyerOrganizationID: r.BuyerOrganizationID, ProviderOrganizationID: r.ProviderOrganizationID, RequestVersion: r.Version, RefundVersion: r.Refund.Version, QuoteVersion: r.Quote.Version, AmountMinor: r.Refund.AmountMinor}, nil
}

type RefundReviewProof struct {
	ReceiptID, InputFingerprint, PaymentReceiptID string
	RemainingMinor                                int64
	ResultFingerprint                             string
}

func (p RefundReviewProof) Fingerprint() string { p.ResultFingerprint = ""; return Fingerprint(p) }
func (p RefundReviewProof) Matches(in RefundReviewAdmission) bool {
	return p.ReceiptID != "" && len(p.ReceiptID) <= 128 && p.InputFingerprint == Fingerprint(in) && p.PaymentReceiptID == in.PaymentReceiptID && p.RemainingMinor >= in.AmountMinor && p.ResultFingerprint == p.Fingerprint()
}

type RefundReviewTradingPort interface {
	AdmitServiceRefundReview(context.Context, RefundReviewAdmission) (RefundReviewProof, error)
}
