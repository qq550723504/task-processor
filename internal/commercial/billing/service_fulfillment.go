package billing

import (
	"context"
	"task-processor/internal/ledger/money"
)

type ServiceFulfillmentCommand struct {
	OrderID, RequestID, PaymentReceiptID, BuyerOrganizationID, ProviderOrganizationID string
	OrganizationID, ActorID, Kind, Key, CommandFingerprint                            string
	RequestVersion, QuoteVersion                                                      int64
}

func (s *ServicePurchases) AdmitServiceFulfillment(ctx context.Context, c ServiceFulfillmentCommand) (money.ServiceFulfillmentProof, error) {
	var out money.ServiceFulfillmentProof
	original, err := s.source.OriginalServicePurchase(ctx, c.OrderID)
	if err != nil {
		return out, err
	}
	if err := s.source.VerifyServiceCommand(ctx, original); err != nil {
		return out, err
	}
	o, err := s.original(ctx, original)
	if err != nil {
		return out, err
	}
	if c.RequestID != o.Source.RequestID || c.BuyerOrganizationID != o.Source.BuyerOrganizationID || c.ProviderOrganizationID != o.Source.ProviderOrganizationID || c.QuoteVersion != o.Source.QuoteVersion || c.PaymentReceiptID == "" || c.PaymentReceiptID != o.PaymentReceiptID || o.Payment == nil {
		return out, ErrConflict
	}
	store, ok := s.funds.(money.ServiceFulfillmentStore)
	if !ok {
		return out, ErrFeatureUnavailable
	}
	in := money.ServiceFulfillmentAdmission{Payment: o.MoneyInput(), PaymentReceiptID: c.PaymentReceiptID, OrganizationID: c.OrganizationID, ActorID: c.ActorID, Kind: c.Kind, Key: c.Key, CommandFingerprint: c.CommandFingerprint, RequestVersion: c.RequestVersion, QuoteVersion: c.QuoteVersion}
	out, err = store.AdmitServiceFulfillment(ctx, in)
	if err != nil {
		return money.ServiceFulfillmentProof{}, err
	}
	if !out.Matches(in) {
		return money.ServiceFulfillmentProof{}, ErrConflict
	}
	return out, nil
}
