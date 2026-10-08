package billing

import (
	"context"
	"task-processor/internal/ledger/money"
)

// Expiry is a reason to verify original channel funds, never an acceptance fact.
func (s *ServicePurchases) observeExpiredFunds(ctx context.Context, o *ServicePurchaseOrder) (bool, error) {
	if o.PaymentReceiptID == "" {
		return true, nil
	}
	f, err := s.funds.ReadServiceFunds(ctx, o.Source.OrderID)
	if err != nil {
		return false, err
	}
	if f.ReconciliationReason == "" && o.FundsExpireAt != nil && !s.now().Before(*o.FundsExpireAt) && f.ExpectedUnsplitMinor() > 0 {
		provider, ok := s.provider.(ServiceUnsplitProvider)
		if !ok {
			return false, ErrFeatureUnavailable
		}
		p, e := provider.QueryServiceUnsplit(ctx, *o)
		if e != nil {
			return false, e
		}
		if !p.Matches(*o) {
			return false, ErrConflict
		}
		f, err = s.funds.ObserveServiceUnsplit(ctx, money.ServiceUnsplitObservation{Payment: o.MoneyInput(), ProfileVersion: p.ProfileVersion, ExpectedFundsFingerprint: money.ServiceFingerprint(f), UnsplitMinor: p.UnsplitMinor, ProofID: p.ProofID, OccurredAt: p.OccurredAt})
		if err != nil {
			return false, err
		}
	}
	return s.projectServiceFundsFence(ctx, o, f)
}

func (s *ServicePurchases) projectServiceFundsFence(ctx context.Context, o *ServicePurchaseOrder, f money.ServiceFundsView) (bool, error) {
	if f.ReconciliationReason != "" {
		if o.State != "RECONCILIATION_REQUIRED" || o.Reason != f.ReconciliationReason {
			o.State = "RECONCILIATION_REQUIRED"
			o.Reason = f.ReconciliationReason
			if err := s.save(ctx, o); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	return true, nil
}

func serviceReleaseOperation(op ServiceFinancialOperation) bool {
	return op.Reservation.Kind == money.ServiceShare || op.Reservation.Kind == money.ServiceFinish || op.Reservation.Kind == money.ServiceRefundRelease
}
