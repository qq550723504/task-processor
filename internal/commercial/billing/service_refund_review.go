package billing

import (
	"context"
	"task-processor/internal/ledger/money"
)

type ServiceRefundReviewCommand struct {
	OrderID, RequestID, PaymentReceiptID, BuyerOrganizationID, ProviderOrganizationID string
	OrganizationID, ActorID, Kind, Key, CommandFingerprint                            string
	RequestVersion, RefundVersion, QuoteVersion, AmountMinor                          int64
}

func (s *ServicePurchases) AdmitServiceRefundReview(ctx context.Context, c ServiceRefundReviewCommand) (money.ServiceRefundReviewProof, error) {
	var out money.ServiceRefundReviewProof
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
	store, ok := s.funds.(money.ServiceRefundReviewStore)
	if !ok {
		return out, ErrFeatureUnavailable
	}
	in := money.ServiceRefundReviewAdmission{Payment: o.MoneyInput(), PaymentReceiptID: c.PaymentReceiptID, OrganizationID: c.OrganizationID, ActorID: c.ActorID, Kind: c.Kind, Key: c.Key, CommandFingerprint: c.CommandFingerprint, RequestVersion: c.RequestVersion, RefundVersion: c.RefundVersion, QuoteVersion: c.QuoteVersion, AmountMinor: c.AmountMinor}
	out, err = store.AdmitServiceRefundReview(ctx, in)
	if err != nil {
		return money.ServiceRefundReviewProof{}, err
	}
	if !out.Matches(in) {
		return money.ServiceRefundReviewProof{}, ErrConflict
	}
	return out, nil
}
