package billing

import "task-processor/internal/ledger/money"

func channelRefundPlan(o ServicePurchaseOrder, c ServicePurchaseCommand, f money.ServiceFundsView) (money.ServiceRefundPlan, error) {
	if p, ok := o.RefundPlans[c.ID]; ok {
		if p.Validate() != nil || p.CommandID != c.ID || p.SourceProofID != c.SourceProofID || p.NominalMinor != c.AmountMinor || p.Allocation != c.Allocation || p.OrderID != c.OrderID || p.PaymentID != f.PaymentID {
			return p, ErrConflict
		}
		return p, nil
	}
	id := func(phase string) string { return "service-operation:" + serviceProviderID(c.ID, phase) }
	return money.NewServiceRefundPlan(f, c.ID, c.SourceProofID, id(money.ServiceReturnPre), "service-operation:"+serviceOperationRequestID(o, c, money.ServiceRefund), id(money.ServiceReturnPost), "service-operation:"+serviceOperationRequestID(o, c, money.ServiceRefundRelease), o.ShareOperationID, c.AmountMinor)
}
func nextChannelRefund(o ServicePurchaseOrder, c ServicePurchaseCommand, f money.ServiceFundsView) (*ServiceFinancialOperation, error) {
	p, err := channelRefundPlan(o, c, f)
	if err != nil {
		return nil, err
	}
	makeOp := func(phase string, kind money.ServiceEffectKind, id string, amount int64) *ServiceFinancialOperation {
		op := &ServiceFinancialOperation{CommandID: c.ID, ProviderRequestID: id[len("service-operation:"):], Reservation: money.ServiceOperation{RefundPlan: &p, RefundPhase: phase, OrderID: c.OrderID, OperationID: id, SourceProofID: c.SourceProofID, Kind: kind, AmountMinor: amount}}
		if kind == money.ServiceReturn {
			op.Reservation.OriginalShareID = p.OriginalShareID
			op.OriginalShareRequestID = o.ShareProviderRequestID
		}
		return op
	}
	if _, ok := o.Effects[p.PreOperationID]; !ok {
		return makeOp(money.ServiceReturnPre, money.ServiceReturn, p.PreOperationID, p.PreReturnMinor), nil
	}
	refund, ok := o.Effects[p.RefundOperationID]
	if !ok {
		if f.SharedMinor > 0 && f.ReleasedMinor == 0 && f.AutomaticReleasedMinor == 0 {
			if _, done := o.Effects[p.ReleaseOperationID]; !done {
				return makeOp(money.ServiceRefundReleasePhase, money.ServiceRefundRelease, p.ReleaseOperationID, f.ExpectedUnsplitMinor()), nil
			}
		}
		return makeOp(money.ServiceRefundPhase, money.ServiceRefund, p.RefundOperationID, p.NominalMinor), nil
	}
	if refund.RefundAmounts == nil {
		return nil, ErrConflict
	}
	if _, ok := o.Effects[p.PostOperationID]; !ok {
		due, err := p.PostReturn(*refund.RefundAmounts)
		if err != nil {
			return nil, err
		}
		return makeOp(money.ServiceReturnPost, money.ServiceReturn, p.PostOperationID, due), nil
	}
	if f.PendingRefundCommandID != "" {
		return nil, ErrConflict
	}
	return nil, nil
}
