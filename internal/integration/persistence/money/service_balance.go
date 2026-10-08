package money

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	m "task-processor/internal/ledger/money"
)

func (r *Repository) ObserveServiceUnsplit(ctx context.Context, in m.ServiceUnsplitObservation) (m.ServiceFundsView, error) {
	var out m.ServiceFundsView
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	in.OccurredAt = m.NormalizeTimestamp(in.OccurredAt)
	fingerprint := m.ServiceFingerprint(in)
	identity := "service-unsplit:" + fingerprint
	stale := false
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
			if prior.Fingerprint != fingerprint {
				return m.ErrConflict
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			payload, e := json.Marshal(in)
			if e != nil {
				return e
			}
			if e = tx.Create(&serviceEffectRow{OperationID: identity, OrderID: row.OrderID, Kind: "UNSPLIT_OBSERVATION", Fingerprint: fingerprint, Receipt: payload}).Error; e != nil {
				return e
			}
		}
		out, err = serviceFunds(row)
		if err != nil {
			return err
		}
		if m.ServiceFingerprint(out) != in.ExpectedFundsFingerprint {
			stale = true
			return nil
		}
		if out.ExpectedUnsplitMinor() != in.UnsplitMinor && row.ReconciliationReason == "" {
			if row.PendingOperationID != "" {
				var pending serviceReservationRow
				if err = tx.Where("operation_id=?", row.PendingOperationID).Take(&pending).Error; err != nil {
					return err
				}
				// The difference may be this original operation, not a release.
				if pending.Dispatched {
					stale = true
					return nil
				}
			}
			row.ReconciliationReason = "CHANNEL_FUNDS_CHANGED_REQUIRES_RECONCILIATION"
			if err = tx.Model(&row).Update("reconciliation_reason", row.ReconciliationReason).Error; err != nil {
				return err
			}
			out, err = serviceFunds(row)
			if err != nil {
				return err
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err == nil && stale {
		err = m.ErrConflict
	}
	return out, err
}
