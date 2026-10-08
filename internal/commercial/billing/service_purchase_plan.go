package billing

import (
	"strconv"
	"task-processor/internal/ledger/money"
)

func serviceOperationRequestID(o ServicePurchaseOrder, c ServicePurchaseCommand, kind money.ServiceEffectKind) string {
	generation := o.AttemptGenerations[c.ID+":"+string(kind)]
	if generation == 0 {
		return serviceProviderID(c.ID, string(kind))
	}
	return serviceProviderID(c.ID, string(kind), strconv.Itoa(generation))
}

func nextServiceOperation(o ServicePurchaseOrder, c ServicePurchaseCommand, f money.ServiceFundsView) (*ServiceFinancialOperation, error) {
	if f.ReconciliationReason != "" {
		return nil, ErrReconciliationRequired
	}
	makeOp := func(kind money.ServiceEffectKind, amount int64) *ServiceFinancialOperation {
		request := serviceOperationRequestID(o, c, kind)
		id := "service-operation:" + request
		return &ServiceFinancialOperation{Reservation: money.ServiceOperation{OrderID: o.Source.OrderID, OperationID: id, Kind: kind, AmountMinor: amount, SourceProofID: c.SourceProofID}, CommandID: c.ID, ProviderRequestID: request}
	}
	done := func(kind money.ServiceEffectKind) bool {
		_, ok := o.Effects["service-operation:"+serviceOperationRequestID(o, c, kind)]
		return ok
	}
	switch c.Kind {
	case "SETTLE":
		if f.ReleasedMinor > 0 && f.SharedMinor-f.ReturnedMinor == f.PlatformMinor {
			return nil, nil
		}
		if !done(money.ServiceShare) {
			return makeOp(money.ServiceShare, f.PlatformMinor-f.SharedMinor+f.ReturnedMinor), nil
		}
		if !done(money.ServiceFinish) {
			return makeOp(money.ServiceFinish, f.ProviderMinor), nil
		}
		return nil, nil
	case "REFUND", "CANCEL":
		if done(money.ServiceRefund) {
			return nil, nil
		}
		if c.AmountMinor > f.GrossMinor-f.RefundedMinor-f.ChargedBackMinor {
			return nil, ErrInvalid
		}
		p, _, err := money.ServiceAllocation(f.GrossMinor, f.RefundedMinor+f.ChargedBackMinor+c.AmountMinor)
		if err != nil {
			return nil, ErrInvalid
		}
		due := f.SharedMinor - f.ReturnedMinor - p
		if due > 0 {
			if done(money.ServiceReturn) || o.ShareOperationID == "" || o.ShareProviderRequestID == "" {
				return nil, ErrConflict
			}
			op := makeOp(money.ServiceReturn, due)
			op.Reservation.OriginalShareID = o.ShareOperationID
			op.OriginalShareRequestID = o.ShareProviderRequestID
			return op, nil
		}
		if f.SharedMinor > 0 && f.ReleasedMinor == 0 && f.AutomaticReleasedMinor == 0 && !done(money.ServiceRefundRelease) {
			return makeOp(money.ServiceRefundRelease, f.GrossMinor-f.RefundedMinor-f.ChargedBackMinor-f.SharedMinor), nil
		}
		return makeOp(money.ServiceRefund, c.AmountMinor), nil
	}
	return nil, ErrInvalid
}
