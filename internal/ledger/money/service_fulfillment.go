package money

import (
	"context"
	"strings"
	"time"
)

type ServiceFulfillmentAdmission struct {
	Payment                                                                  ServicePaymentInput
	PaymentReceiptID, OrganizationID, ActorID, Kind, Key, CommandFingerprint string
	RequestVersion, QuoteVersion                                             int64
}

func (in ServiceFulfillmentAdmission) Validate() error {
	provider := in.Kind == "start" || in.Kind == "deliver"
	buyer := in.Kind == "accept" || in.Kind == "reject"
	if in.Payment.Validate() != nil || !isCanonicalWalletIdentifier(in.PaymentReceiptID) || !isCanonicalWalletIdentifier(in.Key) || !provider && !buyer || provider && in.OrganizationID != in.Payment.ProviderOrganizationID || buyer && in.OrganizationID != in.Payment.BuyerOrganizationID || in.ActorID == "" || strings.TrimSpace(in.ActorID) != in.ActorID || len(in.ActorID) > 256 || len(in.CommandFingerprint) != 64 || in.RequestVersion < 1 || in.QuoteVersion < 1 {
		return ErrInvalid
	}
	return nil
}
func (in ServiceFulfillmentAdmission) Fingerprint() string {
	return ServiceFingerprint(struct {
		Payment, Receipt, Organization, Actor, Kind, Key, Command string
		RequestVersion, QuoteVersion                              int64
	}{in.Payment.Fingerprint(), in.PaymentReceiptID, in.OrganizationID, in.ActorID, in.Kind, in.Key, in.CommandFingerprint, in.RequestVersion, in.QuoteVersion})
}
func (in ServiceFulfillmentAdmission) Identity() string {
	return "service-fulfillment:" + ServiceFingerprint([]string{in.OrganizationID, in.Kind, in.Key})
}

type ServiceFulfillmentProof struct {
	ReceiptID, InputFingerprint, PaymentReceiptID string
	OccurredAt                                    time.Time
	ResultFingerprint                             string
}

func (p ServiceFulfillmentProof) Fingerprint() string {
	p.ResultFingerprint = ""
	return ServiceFingerprint(p)
}
func (p ServiceFulfillmentProof) Matches(in ServiceFulfillmentAdmission) bool {
	return in.Validate() == nil && p.ReceiptID == in.Identity() && p.InputFingerprint == in.Fingerprint() && p.PaymentReceiptID == in.PaymentReceiptID && !p.OccurredAt.IsZero() && p.ResultFingerprint == p.Fingerprint()
}

type ServiceFulfillmentStore interface {
	AdmitServiceFulfillment(context.Context, ServiceFulfillmentAdmission) (ServiceFulfillmentProof, error)
}
