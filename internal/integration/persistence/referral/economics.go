package referral

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	money "task-processor/internal/ledger/money"
	economics "task-processor/internal/referraleconomics"
)

type earningClaim struct {
	PaymentID       string `gorm:"column:payment_id;primaryKey"`
	Issuer          string
	Subject         string
	Referrer        string
	Currency        string
	NetCashMinor    int64
	CommissionMinor int64
	RefundedMinor   int64
	AvailableAt     time.Time
	State           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (earningClaim) TableName() string { return "public.referral_earning_claims" }

type earningLedgerEntry struct {
	EntryID     string `gorm:"column:entry_id;primaryKey"`
	Referrer    string
	Currency    string
	PaymentID   string
	EntryType   string
	AmountMinor int64
	ReferenceID string
	OccurredAt  time.Time
}

func (earningLedgerEntry) TableName() string { return "public.referral_earnings_ledger" }

type refundOperationRow struct {
	PaymentID   string    `gorm:"column:payment_id;primaryKey"`
	RefundID    string    `gorm:"column:refund_id;primaryKey"`
	AmountMinor int64     `gorm:"column:amount_minor"`
	RefundedAt  time.Time `gorm:"column:refunded_at"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (refundOperationRow) TableName() string { return "public.referral_refund_operations" }

type chargebackOperationRow struct {
	PaymentID    string    `gorm:"column:payment_id;primaryKey"`
	ChargebackID string    `gorm:"column:chargeback_id;primaryKey"`
	AmountMinor  int64     `gorm:"column:amount_minor"`
	OccurredAt   time.Time `gorm:"column:occurred_at"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (chargebackOperationRow) TableName() string { return "public.referral_chargeback_operations" }

type earningProjection struct {
	Referrer        string `gorm:"column:referrer;primaryKey"`
	Currency        string `gorm:"column:currency;primaryKey"`
	PendingMinor    int64
	AvailableMinor  int64
	ReservedMinor   int64
	AdjustmentMinor int64
	Version         int64
	UpdatedAt       time.Time
}

func (earningProjection) TableName() string { return "public.referral_earnings_projection" }

type withdrawalRow struct {
	ID              string `gorm:"column:id;primaryKey"`
	Referrer        string
	PayoutMethodID  string
	Currency        string
	Method          string
	AmountMinor     int64
	Status          string
	PayoutReference string
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (withdrawalRow) TableName() string { return "public.referral_withdrawals" }

type withdrawalOperationRow struct {
	IdempotencyKey string `gorm:"column:idempotency_key;primaryKey"`
	WithdrawalID   string
	Fingerprint    string
	ResultVersion  int64
	ResultStatus   string
	CreatedAt      time.Time
}

func (withdrawalOperationRow) TableName() string { return "public.referral_withdrawal_operations" }

type economicsAuditRow struct {
	ID              uint `gorm:"column:id;primaryKey;autoIncrement"`
	Referrer        string
	Actor           string
	ObjectType      string
	ObjectReference string
	Operation       string
	AmountMinor     int64
	IdempotencyKey  string
	CreatedAt       time.Time
}

func (economicsAuditRow) TableName() string { return "public.referral_earnings_audit_events" }

// These read-only mirrors are deliberately private to the referral projection
// boundary. The ledger package owns writes to these tables; referral economics
// may only consume an already durable canonical fact.
type canonicalPaymentRow struct {
	PaymentID                 string
	PayerUserID               string
	Currency                  string
	GrossAmountMinor          int64
	DiscountAmountMinor       int64
	CommissionableAmountMinor int64
	Status                    string
	SettledAt                 time.Time
	ProviderReference         string
	Version                   int64
}

func (canonicalPaymentRow) TableName() string { return "public.ledger_payment_settlements" }

type canonicalRefundRow struct {
	RefundID          string
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

func (canonicalRefundRow) TableName() string { return "public.ledger_refund_settlements" }

type canonicalChargebackRow struct {
	ChargebackID      string
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

func (canonicalChargebackRow) TableName() string { return "public.ledger_chargeback_settlements" }

func (r *Repository) RecordSettledPayment(ctx context.Context, payment money.PaymentSettlement) error {
	if r == nil || r.db == nil || payment.Validate() != nil || payment.Currency != economics.CurrencyCNY {
		return economics.ErrInvalid
	}
	payment.SettledAt = money.NormalizeTimestamp(payment.SettledAt)
	commission, err := economics.CommissionForCashMinor(payment.CommissionableAmountMinor)
	if err != nil {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var canonical canonicalPaymentRow
		if err := tx.Where("payment_id = ? AND payer_user_id = ? AND currency = ? AND gross_amount_minor = ? AND discount_amount_minor = ? AND commissionable_amount_minor = ? AND status = ? AND settled_at = ? AND provider_reference = ? AND version = ?", payment.PaymentID, payment.PayerUserID, payment.Currency, payment.GrossAmountMinor, payment.DiscountAmountMinor, payment.CommissionableAmountMinor, string(payment.Status), payment.SettledAt.UTC(), payment.ProviderReference, payment.Version).Take(&canonical).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrInvalid
		} else if err != nil {
			return economics.ErrUnavailable
		}
		var relations []struct {
			Issuer   string
			Subject  string
			Referrer string
		}
		if err := tx.Table("public.referral_relations").Where("subject=?", payment.PayerUserID).Find(&relations).Error; err != nil {
			return economics.ErrUnavailable
		}
		if len(relations) == 0 {
			return nil
		}
		if len(relations) != 1 {
			return economics.ErrConflict
		}
		relation := relations[0]
		var existing earningClaim
		if err := tx.Where("payment_id=?", payment.PaymentID).Take(&existing).Error; err == nil {
			if existing.Referrer != relation.Referrer || existing.Issuer != relation.Issuer || existing.Subject != payment.PayerUserID || existing.Currency != payment.Currency || existing.CommissionMinor != commission || existing.NetCashMinor != payment.CommissionableAmountMinor || !existing.AvailableAt.Equal(payment.SettledAt.UTC().AddDate(0, 0, 14)) {
				return economics.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrUnavailable
		}
		now := time.Now().UTC()
		claim := earningClaim{PaymentID: payment.PaymentID, Issuer: relation.Issuer, Subject: payment.PayerUserID, Referrer: relation.Referrer, Currency: payment.Currency, NetCashMinor: payment.CommissionableAmountMinor, CommissionMinor: commission, AvailableAt: payment.SettledAt.UTC().AddDate(0, 0, 14), State: "PENDING", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&claim).Error; err != nil {
			return economics.ErrUnavailable
		}
		if err := tx.Create(&earningLedgerEntry{EntryID: uuid.NewString(), Referrer: relation.Referrer, Currency: payment.Currency, PaymentID: payment.PaymentID, EntryType: "COMMISSION", AmountMinor: commission, ReferenceID: payment.PaymentID, OccurredAt: payment.SettledAt.UTC()}).Error; err != nil {
			return economics.ErrUnavailable
		}
		projection, err := lockProjection(tx, relation.Referrer, payment.Currency)
		if err != nil {
			return err
		}
		projection.PendingMinor += commission
		projection.Version++
		projection.UpdatedAt = now
		if err := tx.Save(&projection).Error; err != nil {
			return economics.ErrUnavailable
		}
		return tx.Create(&economicsAuditRow{Referrer: relation.Referrer, Actor: "commercial_owner", ObjectType: "earning", ObjectReference: payment.PaymentID, Operation: "commission_pending", AmountMinor: commission, IdempotencyKey: "payment:" + payment.PaymentID, CreatedAt: now}).Error
	})
}

func (r *Repository) ObservePaymentSettlement(ctx context.Context, payment money.PaymentSettlement) error {
	return r.RecordSettledPayment(ctx, payment)
}

func (r *Repository) RecordRefund(ctx context.Context, refund money.RefundSettlement) error {
	if r == nil || r.db == nil || refund.Validate() != nil {
		return economics.ErrInvalid
	}
	refund.OccurredAt = money.NormalizeTimestamp(refund.OccurredAt)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var canonical canonicalRefundRow
		if err := tx.Where("refund_id = ? AND payment_id = ? AND amount_minor = ? AND occurred_at = ? AND provider_reference = ?", refund.RefundID, refund.PaymentID, refund.AmountMinor, refund.OccurredAt.UTC(), refund.ProviderReference).Take(&canonical).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrInvalid
		} else if err != nil {
			return economics.ErrUnavailable
		}
		var claim earningClaim
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_id=?", refund.PaymentID).Take(&claim).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrInvalid
		} else if err != nil {
			return economics.ErrUnavailable
		}
		if refund.AmountMinor > claim.NetCashMinor-claim.RefundedMinor {
			return economics.ErrInvalid
		}
		var operation refundOperationRow
		if err := tx.Where("payment_id=? AND refund_id=?", refund.PaymentID, refund.RefundID).Take(&operation).Error; err == nil {
			if operation.AmountMinor != refund.AmountMinor || !operation.RefundedAt.Equal(refund.OccurredAt.UTC()) {
				return economics.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrUnavailable
		}
		newRefunded := claim.RefundedMinor + refund.AmountMinor
		adjustment, err := economics.CommissionRefundAdjustment(claim.CommissionMinor, claim.NetCashMinor, claim.RefundedMinor, refund.AmountMinor)
		if err != nil {
			return err
		}
		before := claim.State
		claim.RefundedMinor = newRefunded
		if claim.RefundedMinor == claim.NetCashMinor {
			claim.State = "REVERSED"
		}
		claim.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&claim).Error; err != nil {
			return economics.ErrUnavailable
		}
		if adjustment > 0 {
			if err := tx.Create(&earningLedgerEntry{EntryID: uuid.NewString(), Referrer: claim.Referrer, Currency: claim.Currency, PaymentID: claim.PaymentID, EntryType: "REFUND_ADJUSTMENT", AmountMinor: -adjustment, ReferenceID: refund.RefundID, OccurredAt: refund.OccurredAt.UTC()}).Error; err != nil {
				return economics.ErrUnavailable
			}
		}
		projection, err := lockProjection(tx, claim.Referrer, claim.Currency)
		if err != nil {
			return err
		}
		if adjustment > 0 {
			if before == "PENDING" {
				projection.PendingMinor -= adjustment
			} else {
				projection.AdjustmentMinor -= adjustment
			}
		}
		projection.Version++
		projection.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&projection).Error; err != nil {
			return economics.ErrUnavailable
		}
		if err := tx.Create(&economicsAuditRow{Referrer: claim.Referrer, Actor: "commercial_owner", ObjectType: "earning", ObjectReference: claim.PaymentID, Operation: "refund_adjustment", AmountMinor: -adjustment, IdempotencyKey: "refund:" + refund.RefundID, CreatedAt: time.Now().UTC()}).Error; err != nil {
			return economics.ErrUnavailable
		}
		if err := tx.Create(&refundOperationRow{PaymentID: refund.PaymentID, RefundID: refund.RefundID, AmountMinor: refund.AmountMinor, RefundedAt: refund.OccurredAt.UTC(), CreatedAt: time.Now().UTC()}).Error; err != nil {
			return economics.ErrUnavailable
		}
		return nil
	})
}

func (r *Repository) ObserveRefundSettlement(ctx context.Context, refund money.RefundSettlement) error {
	return r.RecordRefund(ctx, refund)
}

func (r *Repository) RecordChargeback(ctx context.Context, chargeback money.ChargebackSettlement) error {
	if r == nil || r.db == nil || chargeback.Validate() != nil {
		return economics.ErrInvalid
	}
	chargeback.OccurredAt = money.NormalizeTimestamp(chargeback.OccurredAt)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var canonical canonicalChargebackRow
		if err := tx.Where("chargeback_id = ? AND payment_id = ? AND amount_minor = ? AND occurred_at = ? AND provider_reference = ?", chargeback.ChargebackID, chargeback.PaymentID, chargeback.AmountMinor, chargeback.OccurredAt, chargeback.ProviderReference).Take(&canonical).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrInvalid
		} else if err != nil {
			return economics.ErrUnavailable
		}
		var claim earningClaim
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_id=?", chargeback.PaymentID).Take(&claim).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrInvalid
		} else if err != nil {
			return economics.ErrUnavailable
		}
		if chargeback.AmountMinor > claim.NetCashMinor-claim.RefundedMinor {
			return economics.ErrInvalid
		}
		var operation chargebackOperationRow
		if err := tx.Where("payment_id=? AND chargeback_id=?", chargeback.PaymentID, chargeback.ChargebackID).Take(&operation).Error; err == nil {
			if operation.AmountMinor != chargeback.AmountMinor || !operation.OccurredAt.Equal(chargeback.OccurredAt) {
				return economics.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrUnavailable
		}
		before := claim.State
		adjustment, err := economics.CommissionRefundAdjustment(claim.CommissionMinor, claim.NetCashMinor, claim.RefundedMinor, chargeback.AmountMinor)
		if err != nil {
			return err
		}
		claim.RefundedMinor += chargeback.AmountMinor
		if claim.RefundedMinor == claim.NetCashMinor {
			claim.State = "REVERSED"
		}
		now := time.Now().UTC()
		claim.UpdatedAt = now
		if err := tx.Save(&claim).Error; err != nil {
			return economics.ErrUnavailable
		}
		if adjustment > 0 {
			if err := tx.Create(&earningLedgerEntry{EntryID: uuid.NewString(), Referrer: claim.Referrer, Currency: claim.Currency, PaymentID: claim.PaymentID, EntryType: "CHARGEBACK_ADJUSTMENT", AmountMinor: -adjustment, ReferenceID: chargeback.ChargebackID, OccurredAt: chargeback.OccurredAt}).Error; err != nil {
				return economics.ErrUnavailable
			}
		}
		projection, err := lockProjection(tx, claim.Referrer, claim.Currency)
		if err != nil {
			return err
		}
		if adjustment > 0 {
			if before == "PENDING" {
				projection.PendingMinor -= adjustment
			} else {
				projection.AdjustmentMinor -= adjustment
			}
		}
		projection.Version++
		projection.UpdatedAt = now
		if err := tx.Save(&projection).Error; err != nil {
			return economics.ErrUnavailable
		}
		if err := tx.Create(&economicsAuditRow{Referrer: claim.Referrer, Actor: "commercial_owner", ObjectType: "earning", ObjectReference: claim.PaymentID, Operation: "chargeback_adjustment", AmountMinor: -adjustment, IdempotencyKey: "chargeback:" + chargeback.ChargebackID, CreatedAt: now}).Error; err != nil {
			return economics.ErrUnavailable
		}
		return tx.Create(&chargebackOperationRow{PaymentID: chargeback.PaymentID, ChargebackID: chargeback.ChargebackID, AmountMinor: chargeback.AmountMinor, OccurredAt: chargeback.OccurredAt, CreatedAt: now}).Error
	})
}

func (r *Repository) ObserveChargebackSettlement(ctx context.Context, chargeback money.ChargebackSettlement) error {
	return r.RecordChargeback(ctx, chargeback)
}

func (r *Repository) Mature(ctx context.Context, at time.Time) error {
	if r == nil || r.db == nil || at.IsZero() {
		return economics.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var claims []earningClaim
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("state=? AND available_at<=?", "PENDING", at.UTC()).Find(&claims).Error; err != nil {
			return economics.ErrUnavailable
		}
		for _, claim := range claims {
			claim.State = "AVAILABLE"
			claim.UpdatedAt = at.UTC()
			if err := tx.Save(&claim).Error; err != nil {
				return economics.ErrUnavailable
			}
			projection, err := lockProjection(tx, claim.Referrer, claim.Currency)
			if err != nil {
				return err
			}
			refundedCommission, err := economics.CommissionForRefundedCashMinor(claim.CommissionMinor, claim.NetCashMinor, claim.RefundedMinor)
			if err != nil {
				return err
			}
			remaining := claim.CommissionMinor - refundedCommission
			projection.PendingMinor -= remaining
			projection.AvailableMinor += remaining
			projection.Version++
			projection.UpdatedAt = at.UTC()
			if err := tx.Save(&projection).Error; err != nil {
				return economics.ErrUnavailable
			}
		}
		return nil
	})
}

func (r *Repository) ReadEarnings(ctx context.Context, referrer, currency string) (economics.Earnings, error) {
	if r == nil || r.db == nil || referrer == "" || currency != economics.CurrencyCNY {
		return economics.Earnings{}, economics.ErrInvalid
	}
	var p earningProjection
	err := r.db.WithContext(ctx).Where("referrer=? AND currency=?", referrer, currency).Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return economics.Earnings{Referrer: referrer, Currency: currency}, nil
	}
	if err != nil {
		return economics.Earnings{}, economics.ErrUnavailable
	}
	return economics.Earnings{Referrer: p.Referrer, Currency: p.Currency, PendingMinor: p.PendingMinor, AvailableMinor: p.AvailableMinor, ReservedMinor: p.ReservedMinor, AdjustmentMinor: p.AdjustmentMinor, Version: p.Version, UpdatedAt: p.UpdatedAt.UTC()}, nil
}

func (r *Repository) RequestWithdrawal(ctx context.Context, input economics.RequestWithdrawal) (economics.Withdrawal, error) {
	if r == nil || r.db == nil || input.Referrer == "" || input.PayoutMethodID == "" || input.Currency != economics.CurrencyCNY || input.AmountMinor < economics.MinimumWithdrawalMinor || input.IdempotencyKey == "" || (input.Method != economics.MethodAlipay && input.Method != economics.MethodBankTransfer) || input.ExpectedVersion < 0 {
		return economics.Withdrawal{}, economics.ErrInvalid
	}
	var out economics.Withdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var op withdrawalOperationRow
		if err := tx.Where("idempotency_key=?", input.IdempotencyKey).Take(&op).Error; err == nil {
			if op.Fingerprint != withdrawalFingerprint(input) {
				return economics.ErrIdempotencyConflict
			}
			var row withdrawalRow
			if e := tx.Where("id=?", op.WithdrawalID).Take(&row).Error; e != nil {
				return economics.ErrUnavailable
			}
			out = withdrawalFromRow(row)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrUnavailable
		}
		p, err := lockProjection(tx, input.Referrer, input.Currency)
		if err != nil {
			return err
		}
		if p.Version != input.ExpectedVersion {
			return economics.ErrConflict
		}
		// AvailableMinor is already reduced when a withdrawal is reserved;
		// subtracting ReservedMinor again would reject valid later requests.
		if p.AvailableMinor+p.AdjustmentMinor < 0 || p.AvailableMinor+p.AdjustmentMinor < input.AmountMinor {
			return economics.ErrInsufficient
		}
		now := time.Now().UTC()
		id := uuid.NewString()
		row := withdrawalRow{ID: id, Referrer: input.Referrer, PayoutMethodID: input.PayoutMethodID, Currency: input.Currency, Method: string(input.Method), AmountMinor: input.AmountMinor, Status: string(economics.WithdrawalRequested), Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return economics.ErrUnavailable
		}
		p.AvailableMinor -= input.AmountMinor
		p.ReservedMinor += input.AmountMinor
		p.Version++
		p.UpdatedAt = now
		if err := tx.Save(&p).Error; err != nil {
			return economics.ErrUnavailable
		}
		fingerprint := withdrawalFingerprint(input)
		if err := tx.Create(&withdrawalOperationRow{IdempotencyKey: input.IdempotencyKey, WithdrawalID: id, Fingerprint: fingerprint, ResultVersion: 1, ResultStatus: string(economics.WithdrawalRequested), CreatedAt: now}).Error; err != nil {
			return economics.ErrUnavailable
		}
		if err := tx.Create(&economicsAuditRow{Referrer: input.Referrer, Actor: input.Referrer, ObjectType: "withdrawal", ObjectReference: id, Operation: "requested", AmountMinor: -input.AmountMinor, IdempotencyKey: input.IdempotencyKey, CreatedAt: now}).Error; err != nil {
			return economics.ErrUnavailable
		}
		out = withdrawalFromRow(row)
		return nil
	})
	return out, err
}

func (r *Repository) CancelWithdrawal(ctx context.Context, id, referrer string, expected int64, idempotencyKey string) (economics.Withdrawal, error) {
	return r.transitionWithdrawal(ctx, economics.ReviewWithdrawal{WithdrawalID: id, ExpectedVersion: expected, Actor: referrer, Action: economics.WithdrawalCanceled, IdempotencyKey: idempotencyKey}, referrer)
}
func (r *Repository) ReviewWithdrawal(ctx context.Context, input economics.ReviewWithdrawal) (economics.Withdrawal, error) {
	if input.Action != economics.WithdrawalApproved && input.Action != economics.WithdrawalRejected && input.Action != economics.WithdrawalPaid {
		return economics.Withdrawal{}, economics.ErrInvalid
	}
	return r.transitionWithdrawal(ctx, input, "")
}
func (r *Repository) transitionWithdrawal(ctx context.Context, input economics.ReviewWithdrawal, referrer string) (economics.Withdrawal, error) {
	if r == nil || r.db == nil || input.WithdrawalID == "" || input.ExpectedVersion < 1 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return economics.Withdrawal{}, economics.ErrInvalid
	}
	var out economics.Withdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fingerprint := transitionFingerprint(input)
		var op withdrawalOperationRow
		if err := tx.Where("idempotency_key=?", input.IdempotencyKey).Take(&op).Error; err == nil {
			if op.Fingerprint != fingerprint {
				return economics.ErrIdempotencyConflict
			}
			var existing withdrawalRow
			if err := tx.Where("id=?", op.WithdrawalID).Take(&existing).Error; err != nil {
				return economics.ErrUnavailable
			}
			if referrer != "" && existing.Referrer != referrer {
				return economics.ErrConflict
			}
			out = withdrawalFromRow(existing)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return economics.ErrUnavailable
		}
		var row withdrawalRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", input.WithdrawalID).Take(&row).Error; err != nil {
			return economics.ErrUnavailable
		}
		if referrer != "" && row.Referrer != referrer {
			return economics.ErrConflict
		}
		if row.Version != input.ExpectedVersion {
			return economics.ErrConflict
		}
		if input.Action == economics.WithdrawalPaid && strings.TrimSpace(input.ExternalPaymentReference) == "" {
			return economics.ErrInvalid
		}
		valid := row.Status == string(economics.WithdrawalRequested) && (input.Action == economics.WithdrawalApproved || input.Action == economics.WithdrawalRejected || input.Action == economics.WithdrawalCanceled) || row.Status == string(economics.WithdrawalApproved) && input.Action == economics.WithdrawalPaid
		if !valid {
			return economics.ErrInvalidTransition
		}
		now := time.Now().UTC()
		row.Status = string(input.Action)
		row.PayoutReference = strings.TrimSpace(input.ExternalPaymentReference)
		row.Version++
		row.UpdatedAt = now
		if err := tx.Save(&row).Error; err != nil {
			return economics.ErrUnavailable
		}
		if input.Action == economics.WithdrawalRejected || input.Action == economics.WithdrawalCanceled {
			p, err := lockProjection(tx, row.Referrer, row.Currency)
			if err != nil {
				return err
			}
			p.ReservedMinor -= row.AmountMinor
			p.AvailableMinor += row.AmountMinor
			p.Version++
			p.UpdatedAt = now
			if err := tx.Save(&p).Error; err != nil {
				return economics.ErrUnavailable
			}
		} else if input.Action == economics.WithdrawalPaid {
			p, err := lockProjection(tx, row.Referrer, row.Currency)
			if err != nil {
				return err
			}
			// Refund/chargeback adjustments reduce the effective earnings after
			// a withdrawal has reserved funds. Do not let an approved request
			// pay out while the remaining unreserved projection is negative;
			// the manual reviewer must reject/correct it first.
			if p.AvailableMinor+p.AdjustmentMinor < 0 {
				return economics.ErrInsufficient
			}
			if p.ReservedMinor < row.AmountMinor {
				return economics.ErrUnavailable
			}
			p.ReservedMinor -= row.AmountMinor
			p.Version++
			p.UpdatedAt = now
			if err := tx.Save(&p).Error; err != nil {
				return economics.ErrUnavailable
			}
		}
		if err := tx.Create(&withdrawalOperationRow{IdempotencyKey: input.IdempotencyKey, WithdrawalID: row.ID, Fingerprint: fingerprint, ResultVersion: row.Version, ResultStatus: row.Status, CreatedAt: now}).Error; err != nil {
			return economics.ErrUnavailable
		}
		actor := input.Actor
		if actor == "" {
			actor = referrer
		}
		if err := tx.Create(&economicsAuditRow{Referrer: row.Referrer, Actor: actor, ObjectType: "withdrawal", ObjectReference: row.ID, Operation: strings.ToLower(string(input.Action)), AmountMinor: 0, IdempotencyKey: input.IdempotencyKey, CreatedAt: now}).Error; err != nil {
			return economics.ErrUnavailable
		}
		out = withdrawalFromRow(row)
		return nil
	})
	return out, err
}

func lockProjection(tx *gorm.DB, referrer, currency string) (earningProjection, error) {
	var p earningProjection
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("referrer=? AND currency=?", referrer, currency).Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p = earningProjection{Referrer: referrer, Currency: currency, UpdatedAt: time.Now().UTC()}
		if err := tx.Create(&p).Error; err != nil {
			return p, economics.ErrUnavailable
		}
		return p, nil
	}
	if err != nil {
		return p, economics.ErrUnavailable
	}
	return p, nil
}
func withdrawalFromRow(row withdrawalRow) economics.Withdrawal {
	return economics.Withdrawal{ID: row.ID, Referrer: row.Referrer, Currency: row.Currency, Method: economics.WithdrawalMethod(row.Method), PayoutMethodID: row.PayoutMethodID, AmountMinor: row.AmountMinor, Status: economics.WithdrawalStatus(row.Status), PayoutReference: row.PayoutReference, Version: row.Version, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}
func withdrawalFingerprint(input economics.RequestWithdrawal) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d", input.Referrer, input.Currency, input.Method, input.PayoutMethodID, input.AmountMinor, input.ExpectedVersion)))
	return hex.EncodeToString(sum[:])
}

func transitionFingerprint(input economics.ReviewWithdrawal) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s|%s", input.WithdrawalID, input.ExpectedVersion, input.Action, strings.TrimSpace(input.ExternalPaymentReference))))
	return hex.EncodeToString(sum[:])
}
