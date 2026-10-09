package money

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	m "task-processor/internal/ledger/money"
	"time"

	"gorm.io/gorm"
)

type serviceRefundReviewRow struct {
	ReceiptID    string `gorm:"primaryKey;size:128"`
	OrderID      string `gorm:"index;size:128;not null"`
	Fingerprint  string `gorm:"size:64;not null"`
	Input, Proof []byte `gorm:"not null"`
}

func (serviceRefundReviewRow) TableName() string { return "ledger_service_refund_review_admissions" }

func (r *Repository) AdmitServiceRefundReview(ctx context.Context, in m.ServiceRefundReviewAdmission) (m.ServiceRefundReviewProof, error) {
	var out m.ServiceRefundReviewProof
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.Payment.OrderID)
		if err != nil {
			return err
		}
		var payment m.ServiceReceipt
		if row.Fingerprint != in.Payment.Fingerprint() || decodeServiceReceipt(row.Receipt, &payment) != nil || payment.ReceiptID != in.PaymentReceiptID {
			return m.ErrConflict
		}
		var prior serviceRefundReviewRow
		if err := tx.Where("receipt_id=?", in.Identity()).Take(&prior).Error; err == nil {
			var saved m.ServiceRefundReviewAdmission
			if prior.OrderID != in.Payment.OrderID || prior.Fingerprint != in.Fingerprint() || json.Unmarshal(prior.Input, &saved) != nil || saved.Validate() != nil || saved.Fingerprint() != prior.Fingerprint || json.Unmarshal(prior.Proof, &out) != nil || !out.Matches(in) {
				return m.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		funds, err := serviceFunds(row)
		if err != nil {
			return err
		}
		remaining := funds.GrossMinor - funds.RefundedMinor - funds.ChargedBackMinor
		if funds.ReconciliationReason != "" || funds.ChargedBackMinor != 0 || remaining < in.AmountMinor {
			return m.ErrConflict
		}
		out = m.ServiceRefundReviewProof{ReceiptID: in.Identity(), InputFingerprint: in.Fingerprint(), PaymentReceiptID: in.PaymentReceiptID, RemainingMinor: remaining, OccurredAt: m.NormalizeTimestamp(time.Now().UTC())}
		out.ResultFingerprint = out.Fingerprint()
		input, err := json.Marshal(in)
		if err != nil {
			return err
		}
		proof, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Create(&serviceRefundReviewRow{ReceiptID: out.ReceiptID, OrderID: in.Payment.OrderID, Fingerprint: in.Fingerprint(), Input: input, Proof: proof}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return m.ServiceRefundReviewProof{}, err
	}
	return out, nil
}
