package money

import (
	"context"
	"strings"
	"time"
)

// ServiceRefundReviewAdmission is a non-economic amount check for exactly one
// platform review. It is never an operation reservation or dispatch authority.
type ServiceRefundReviewAdmission struct {
	Payment                                                                  ServicePaymentInput
	PaymentReceiptID, OrganizationID, ActorID, Kind, Key, CommandFingerprint string
	RequestVersion, RefundVersion, QuoteVersion, AmountMinor                 int64
}

func (in ServiceRefundReviewAdmission) Validate() error {
	if in.Payment.Validate() != nil || !isCanonicalWalletIdentifier(in.PaymentReceiptID) || !isCanonicalWalletIdentifier(in.Key) || in.Kind != "refund_review" || in.ActorID == "" || strings.TrimSpace(in.ActorID) != in.ActorID || len(in.ActorID) > 256 || len(in.OrganizationID) > 128 || len(in.CommandFingerprint) != 64 || in.RequestVersion < 1 || in.RefundVersion < 1 || in.QuoteVersion < 1 || in.AmountMinor <= 0 || in.AmountMinor > in.Payment.Payment.GrossAmountMinor {
		return ErrInvalid
	}
	return nil
}
func (in ServiceRefundReviewAdmission) Fingerprint() string {
	return ServiceFingerprint(struct {
		Payment, Receipt, Organization, Actor, Kind, Key, Command string
		RequestVersion, RefundVersion, QuoteVersion, Amount       int64
	}{in.Payment.Fingerprint(), in.PaymentReceiptID, in.OrganizationID, in.ActorID, in.Kind, in.Key, in.CommandFingerprint, in.RequestVersion, in.RefundVersion, in.QuoteVersion, in.AmountMinor})
}
func (in ServiceRefundReviewAdmission) Identity() string {
	return "service-refund-review:" + ServiceFingerprint([]string{in.OrganizationID, in.Kind, in.Key})
}

type ServiceRefundReviewProof struct {
	ReceiptID, InputFingerprint, PaymentReceiptID string
	RemainingMinor                                int64
	OccurredAt                                    time.Time
	ResultFingerprint                             string
}

func (p ServiceRefundReviewProof) Fingerprint() string {
	p.ResultFingerprint = ""
	return ServiceFingerprint(p)
}
func (p ServiceRefundReviewProof) Matches(in ServiceRefundReviewAdmission) bool {
	return in.Validate() == nil && p.ReceiptID == in.Identity() && p.InputFingerprint == in.Fingerprint() && p.PaymentReceiptID == in.PaymentReceiptID && p.RemainingMinor >= in.AmountMinor && p.RemainingMinor <= in.Payment.Payment.GrossAmountMinor && !p.OccurredAt.IsZero() && p.ResultFingerprint == p.Fingerprint()
}

type ServiceRefundReviewStore interface {
	AdmitServiceRefundReview(context.Context, ServiceRefundReviewAdmission) (ServiceRefundReviewProof, error)
}
