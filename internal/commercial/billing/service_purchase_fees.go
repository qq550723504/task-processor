package billing

import (
	"context"
	"task-processor/internal/ledger/money"
)

type ServiceFeeProvider interface {
	QueryServiceFees(context.Context, ServicePurchaseOrder, string) ([]money.ServiceChannelFee, error)
}
type ServiceFinancialView struct {
	GrossMinor           int64  `json:"grossMinor,string"`
	RefundedMinor        int64  `json:"refundedMinor,string"`
	PlatformMinor        int64  `json:"platformMinor,string"`
	ProviderMinor        int64  `json:"providerMinor,string"`
	SharedMinor          int64  `json:"sharedMinor,string"`
	ReturnedMinor        int64  `json:"returnedMinor,string"`
	ReleasedMinor        int64  `json:"releasedMinor,string"`
	ChannelFeeMinor      int64  `json:"channelFeeMinor,string"`
	ChannelFeeObserved   bool   `json:"channelFeeObserved"`
	ReconciliationReason string `json:"reconciliationReason"`
}

func (s *ServicePurchases) ReadFinancialFacts(ctx context.Context, orderID string) (ServiceFinancialView, error) {
	f, err := s.funds.ReadServiceFunds(ctx, orderID)
	return ServiceFinancialView{GrossMinor: f.GrossMinor, RefundedMinor: f.RefundedMinor, PlatformMinor: f.PlatformMinor, ProviderMinor: f.ProviderMinor, SharedMinor: f.SharedMinor, ReturnedMinor: f.ReturnedMinor, ReleasedMinor: f.ReleasedMinor, ChannelFeeMinor: f.ChannelFeeMinor, ChannelFeeObserved: f.ChannelFeeObserved, ReconciliationReason: f.ReconciliationReason}, err
}

// The platform consumer supplies a bounded statement date, never an amount,
// merchant, raw statement, transaction or external download address.
func (s *ServicePurchases) RefreshChannelFees(ctx context.Context, orderID, date string) (ServiceFinancialView, error) {
	o, err := s.store.ReadServicePurchase(ctx, orderID)
	if err != nil {
		return ServiceFinancialView{}, err
	}
	if o.Profile != s.provider.Profile() || o.Payment == nil || o.PaymentReceiptID == "" {
		return ServiceFinancialView{}, ErrFeatureUnavailable
	}
	provider, ok := s.provider.(ServiceFeeProvider)
	if !ok {
		return ServiceFinancialView{}, ErrFeatureUnavailable
	}
	facts, err := provider.QueryServiceFees(ctx, o, date)
	if err != nil {
		return ServiceFinancialView{}, err
	}
	for _, fact := range facts {
		if fact.Payment.Fingerprint() != o.MoneyInput().Fingerprint() {
			return ServiceFinancialView{}, ErrConflict
		}
		if _, err = s.funds.ObserveServiceChannelFee(ctx, fact); err != nil {
			return ServiceFinancialView{}, err
		}
	}
	return s.ReadFinancialFacts(ctx, orderID)
}
