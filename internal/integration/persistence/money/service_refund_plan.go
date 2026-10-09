package money

import (
	"encoding/json"
	"gorm.io/gorm"
	m "task-processor/internal/ledger/money"
)

func validateServiceRefundPlan(tx *gorm.DB, row servicePaymentRow, op m.ServiceOperation, f m.ServiceFundsView) error {
	if f.Allocation.Basis != m.ServiceAllocationChannelNetFloorV2 {
		if op.RefundPlan != nil {
			return m.ErrConflict
		}
		return nil
	}
	if op.RefundPlan == nil {
		if len(row.PendingRefundPlan) > 0 || op.Kind == m.ServiceRefund || op.Kind == m.ServiceReturn || op.Kind == m.ServiceRefundRelease {
			return m.ErrConflict
		}
		return nil
	}
	p := *op.RefundPlan
	if len(row.PendingRefundPlan) == 0 {
		if op.RefundPhase != m.ServiceReturnPre {
			return m.ErrConflict
		}
		expected, err := m.NewServiceRefundPlan(f, p.CommandID, p.SourceProofID, p.PreOperationID, p.RefundOperationID, p.PostOperationID, p.ReleaseOperationID, p.OriginalShareID, p.NominalMinor)
		if err != nil || expected != p {
			return m.ErrConflict
		}
	} else {
		var saved m.ServiceRefundPlan
		if json.Unmarshal(row.PendingRefundPlan, &saved) != nil || saved != p {
			return m.ErrConflict
		}
	}
	proof := func(id string, kind m.ServiceEffectKind) (m.ServiceReceipt, error) {
		var e serviceEffectRow
		var r m.ServiceReceipt
		if tx.Where("operation_id=? AND order_id=? AND kind=?", id, row.OrderID, string(kind)).Take(&e).Error != nil || decodeServiceReceipt(e.Receipt, &r) != nil {
			return r, m.ErrConflict
		}
		return r, nil
	}
	switch op.RefundPhase {
	case m.ServiceReturnPre:
		if row.RefundedMinor != p.BaseRefundedMinor || row.SettlementRefundedMinor != p.BaseSettlementRefundedMinor || row.ReturnedMinor != p.BaseReturnedMinor {
			return m.ErrConflict
		}
	case m.ServiceRefundReleasePhase, m.ServiceRefundPhase:
		if _, err := proof(p.PreOperationID, m.ServiceReturn); err != nil {
			return err
		}
		if row.RefundedMinor != p.BaseRefundedMinor || row.SettlementRefundedMinor != p.BaseSettlementRefundedMinor || row.ReturnedMinor != p.BaseReturnedMinor+p.PreReturnMinor {
			return m.ErrConflict
		}
		if op.RefundPhase == m.ServiceRefundPhase && row.SharedMinor > 0 && row.ReleasedMinor == 0 && row.AutomaticReleasedMinor == 0 {
			return m.ErrConflict
		}
	case m.ServiceReturnPost:
		receipt, err := proof(p.RefundOperationID, m.ServiceRefund)
		if err != nil || receipt.RefundAmounts == nil {
			return m.ErrConflict
		}
		due, err := p.PostReturn(*receipt.RefundAmounts)
		if err != nil || op.AmountMinor != due || row.RefundedMinor != p.BaseRefundedMinor+p.NominalMinor || row.SettlementRefundedMinor != p.BaseSettlementRefundedMinor+receipt.RefundAmounts.SettlementMinor() || row.ReturnedMinor != p.BaseReturnedMinor+p.PreReturnMinor {
			return m.ErrConflict
		}
	default:
		return m.ErrConflict
	}
	return nil
}

func acceptServiceRefundAmounts(row *servicePaymentRow, e m.ServiceEffect) error {
	policy, err := serviceAllocationPolicy(*row)
	if err != nil {
		return err
	}
	if e.Operation.Kind != m.ServiceRefund {
		return nil
	}
	if policy.Basis != m.ServiceAllocationChannelNetFloorV2 {
		if e.RefundAmounts != nil {
			return m.ErrConflict
		}
		return nil
	}
	var original m.ServicePaymentInput
	if json.Unmarshal(row.Input, &original) != nil || original.ChannelAmounts == nil || e.RefundAmounts == nil || e.RefundAmounts.Validate(*original.ChannelAmounts, e.Operation.AmountMinor) != nil {
		return m.ErrConflict
	}
	a := e.RefundAmounts.Normalize()
	e.RefundAmounts = &a
	if a.PayerMinor > original.ChannelAmounts.PayerMinor-row.PayerRefundedMinor || a.SettlementMinor() > original.ChannelAmounts.SettlementMinor()-row.SettlementRefundedMinor {
		return m.ErrConflict
	}
	used := map[string]int64{}
	if len(row.VoucherRefunded) > 0 && json.Unmarshal(row.VoucherRefunded, &used) != nil {
		return m.ErrConflict
	}
	for _, v := range a.Vouchers {
		if v.RefundMinor > v.OriginalAmountMinor-used[v.ID] {
			return m.ErrConflict
		}
		used[v.ID] += v.RefundMinor
	}
	row.PayerRefundedMinor += a.PayerMinor
	row.SettlementRefundedMinor += a.SettlementMinor()
	row.VoucherRefunded, err = json.Marshal(used)
	return err
}
