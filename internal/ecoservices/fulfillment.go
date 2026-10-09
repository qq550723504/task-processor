package ecoservices

import "context"

// FulfillmentAdmission binds the original command to a durable check at M.
// It is not authority to dispatch any financial operation.
type FulfillmentAdmission struct {
	Scope                                                               Scope
	Key, Kind, RequestID, OrderID, PaymentReceiptID, CommandFingerprint string
	BuyerOrganizationID, ProviderOrganizationID                         string
	RequestVersion, QuoteVersion                                        int64
}

func IsFulfillment(kind string) bool {
	return kind == "start" || kind == "deliver" || kind == "accept" || kind == "reject"
}

func BuildFulfillmentAdmission(c Command, r Request) (FulfillmentAdmission, error) {
	if c.Scope.Platform || !IsFulfillment(c.Kind) {
		return FulfillmentAdmission{}, ErrForbidden
	}
	wantOrg := r.ProviderOrganizationID
	if c.Kind == "accept" || c.Kind == "reject" {
		wantOrg = r.BuyerOrganizationID
	}
	if c.Scope.OrganizationID != wantOrg {
		return FulfillmentAdmission{}, ErrForbidden
	}
	copyCommand := c
	copyCommand.Fingerprint = ""
	if !ValidID(c.Key) || c.ID != r.ID || c.Version != r.Version || c.Fingerprint != Fingerprint(copyCommand) || r.Quote == nil || r.PaymentReceiptID == "" || r.FinancialFence || r.FinancialState == "RECONCILIATION_REQUIRED" {
		return FulfillmentAdmission{}, ErrConflict
	}
	return FulfillmentAdmission{Scope: c.Scope, Key: c.Key, Kind: c.Kind, RequestID: r.ID, OrderID: r.OrderID, PaymentReceiptID: r.PaymentReceiptID, CommandFingerprint: c.Fingerprint, BuyerOrganizationID: r.BuyerOrganizationID, ProviderOrganizationID: r.ProviderOrganizationID, RequestVersion: r.Version, QuoteVersion: r.Quote.Version}, nil
}

type FulfillmentProof struct {
	ReceiptID, InputFingerprint, PaymentReceiptID, ResultFingerprint string
}

func (p FulfillmentProof) Fingerprint() string { p.ResultFingerprint = ""; return Fingerprint(p) }
func (p FulfillmentProof) Matches(in FulfillmentAdmission) bool {
	return p.ReceiptID != "" && len(p.ReceiptID) <= 128 && p.InputFingerprint == Fingerprint(in) && p.PaymentReceiptID == in.PaymentReceiptID && p.ResultFingerprint == p.Fingerprint()
}

type FulfillmentTradingPort interface {
	AdmitServiceFulfillment(context.Context, FulfillmentAdmission) (FulfillmentProof, error)
}
