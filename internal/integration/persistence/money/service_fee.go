package money

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"math"
	m "task-processor/internal/ledger/money"
)

func (r *Repository) ObserveServiceChannelFee(ctx context.Context, in m.ServiceChannelFee) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	in.OccurredAt = m.NormalizeTimestamp(in.OccurredAt)
	identity := "service-fee:" + m.ServiceFingerprint([]string{in.Payment.Binding.Provider, in.Payment.Binding.Environment, in.Payment.PlatformMerchantID, in.FlowID})
	fp := in.Fingerprint()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.Payment.OrderID)
		if err != nil {
			return err
		}
		if row.Fingerprint != in.Payment.Fingerprint() {
			return m.ErrConflict
		}
		var prior serviceEffectRow
		if err = tx.Where("operation_id=?", identity).Take(&prior).Error; err == nil {
			if prior.OrderID != row.OrderID || prior.Fingerprint != fp {
				return m.ErrConflict
			}
			return decodeServiceReceipt(prior.Receipt, &out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if in.BusinessID != in.Payment.Binding.TradeID {
			var refunds []serviceEffectRow
			if err = tx.Where("order_id=? AND kind=?", row.OrderID, string(m.ServiceRefund)).Find(&refunds).Error; err != nil {
				return err
			}
			bound := false
			for _, refund := range refunds {
				var fact m.ServiceReceipt
				if decodeServiceReceipt(refund.Receipt, &fact) != nil {
					return m.ErrConflict
				}
				if fact.ProviderReference == "wechat-refund:"+in.BusinessID {
					bound = true
				}
			}
			if !bound {
				return m.ErrConflict
			}
		}
		kind := "CHANNEL_FEE"
		if in.Returned {
			kind = "CHANNEL_FEE_RETURN"
			if in.AmountMinor > row.ChannelFeeMinor {
				return m.ErrConflict
			}
			row.ChannelFeeMinor -= in.AmountMinor
		} else {
			if in.AmountMinor > math.MaxInt64-row.ChannelFeeMinor {
				return m.ErrConflict
			}
			row.ChannelFeeMinor += in.AmountMinor
		}
		out = m.ServiceReceipt{ReceiptID: identity, OrderID: row.OrderID, OperationID: identity, Kind: m.ServiceEffectKind(kind), AmountMinor: in.AmountMinor, RequestFingerprint: fp, ProviderReference: in.ProofID, OccurredAt: in.OccurredAt}
		out.ResultFingerprint = out.Fingerprint()
		payload, err := json.Marshal(struct {
			m.ServiceReceipt
			SourceFee m.ServiceChannelFee
		}{out, in})
		if err != nil {
			return err
		}
		if err = tx.Create(&serviceEffectRow{OperationID: identity, OrderID: row.OrderID, Kind: kind, Fingerprint: fp, Receipt: payload}).Error; err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"channel_fee_minor": row.ChannelFeeMinor, "channel_fee_observed": true}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
