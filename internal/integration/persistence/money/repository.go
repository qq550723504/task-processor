package money

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	money "task-processor/internal/ledger/money"
)

type paymentRow struct {
	PaymentID                 string `gorm:"column:payment_id;primaryKey"`
	PayerUserID               string `gorm:"column:payer_user_id"`
	Currency                  string
	GrossAmountMinor          int64
	DiscountAmountMinor       int64
	CommissionableAmountMinor int64
	Status                    string
	SettledAt                 time.Time
	ProviderReference         string
	Version                   int64
}

func (paymentRow) TableName() string { return "ledger_payment_settlements" }

type refundRow struct {
	RefundID          string `gorm:"column:refund_id;primaryKey"`
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

type chargebackRow struct {
	ChargebackID      string `gorm:"column:chargeback_id;primaryKey"`
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

func (chargebackRow) TableName() string { return "ledger_chargeback_settlements" }

func (refundRow) TableName() string { return "ledger_refund_settlements" }

type payoutMethodRow struct {
	MethodID          string `gorm:"column:method_id;primaryKey"`
	SubjectUserID     string
	Type              string
	DisplayName       string
	MaskedDestination string
	SecureReference   []byte
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
}

type payoutMethodOperationRow struct {
	IdempotencyKey string `gorm:"column:idempotency_key;primaryKey"`
	MethodID       string
	Fingerprint    string
	CreatedAt      time.Time
}

func (payoutMethodOperationRow) TableName() string { return "ledger_payout_method_operations" }

func (payoutMethodRow) TableName() string { return "ledger_payout_methods" }

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, money.ErrUnavailable
	}
	return &Repository{db: db}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return money.ErrUnavailable
	}
	return db.AutoMigrate(&paymentRow{}, &refundRow{}, &chargebackRow{}, &payoutMethodRow{}, &payoutMethodOperationRow{})
}

func (r *Repository) RecordPaymentSettlement(ctx context.Context, payment money.PaymentSettlement) error {
	if r == nil || r.db == nil || payment.Validate() != nil {
		return money.ErrInvalid
	}
	row := paymentRow{PaymentID: payment.PaymentID, PayerUserID: payment.PayerUserID, Currency: payment.Currency, GrossAmountMinor: payment.GrossAmountMinor, DiscountAmountMinor: payment.DiscountAmountMinor, CommissionableAmountMinor: payment.CommissionableAmountMinor, Status: string(payment.Status), SettledAt: money.NormalizeTimestamp(payment.SettledAt), ProviderReference: payment.ProviderReference, Version: payment.Version}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing paymentRow
		err := tx.Where("payment_id = ?", row.PaymentID).Take(&existing).Error
		if err == nil {
			if existing.PayerUserID != row.PayerUserID || existing.Currency != row.Currency || existing.GrossAmountMinor != row.GrossAmountMinor || existing.DiscountAmountMinor != row.DiscountAmountMinor || existing.CommissionableAmountMinor != row.CommissionableAmountMinor || existing.Status != row.Status || !existing.SettledAt.Equal(row.SettledAt) || existing.ProviderReference != row.ProviderReference || existing.Version != row.Version {
				return money.ErrConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return money.ErrUnavailable
		}
		if err := tx.Create(&row).Error; err != nil {
			return money.ErrUnavailable
		}
		return nil
	})
}

func (r *Repository) RecordPaymentSettlementAndNotify(ctx context.Context, payment money.PaymentSettlement, observer money.SettlementObserver) error {
	if observer == nil {
		return money.ErrInvalid
	}
	if err := r.RecordPaymentSettlement(ctx, payment); err != nil {
		return err
	}
	return observer.ObservePaymentSettlement(ctx, payment)
}

func (r *Repository) RecordRefundSettlement(ctx context.Context, refund money.RefundSettlement) error {
	if r == nil || r.db == nil || refund.Validate() != nil {
		return money.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment paymentRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_id = ?", refund.PaymentID).Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return money.ErrNotFound
			}
			return money.ErrUnavailable
		}
		var existing refundRow
		err := tx.Where("refund_id = ?", refund.RefundID).Take(&existing).Error
		if err == nil {
			if existing.PaymentID != refund.PaymentID || existing.AmountMinor != refund.AmountMinor || !existing.OccurredAt.Equal(money.NormalizeTimestamp(refund.OccurredAt)) || existing.ProviderReference != refund.ProviderReference {
				return money.ErrConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return money.ErrUnavailable
		}
		var refunded int64
		if err := tx.Model(&refundRow{}).Where("payment_id = ?", refund.PaymentID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&refunded).Error; err != nil {
			return money.ErrUnavailable
		}
		var chargedBack int64
		if err := tx.Model(&chargebackRow{}).Where("payment_id = ?", refund.PaymentID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&chargedBack).Error; err != nil {
			return money.ErrUnavailable
		}
		if refund.AmountMinor > payment.CommissionableAmountMinor-refunded-chargedBack {
			return money.ErrInvalid
		}
		if err := tx.Create(&refundRow{RefundID: refund.RefundID, PaymentID: refund.PaymentID, AmountMinor: refund.AmountMinor, OccurredAt: money.NormalizeTimestamp(refund.OccurredAt), ProviderReference: refund.ProviderReference}).Error; err != nil {
			return money.ErrUnavailable
		}
		return nil
	})
}

func (r *Repository) RecordChargebackSettlement(ctx context.Context, chargeback money.ChargebackSettlement) error {
	if r == nil || r.db == nil || chargeback.Validate() != nil {
		return money.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment paymentRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_id = ?", chargeback.PaymentID).Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return money.ErrNotFound
			}
			return money.ErrUnavailable
		}
		var existing chargebackRow
		err := tx.Where("chargeback_id = ?", chargeback.ChargebackID).Take(&existing).Error
		if err == nil {
			if existing.PaymentID != chargeback.PaymentID || existing.AmountMinor != chargeback.AmountMinor || !existing.OccurredAt.Equal(money.NormalizeTimestamp(chargeback.OccurredAt)) || existing.ProviderReference != chargeback.ProviderReference {
				return money.ErrConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return money.ErrUnavailable
		}
		var refunded, chargedBack int64
		if err := tx.Model(&refundRow{}).Where("payment_id = ?", chargeback.PaymentID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&refunded).Error; err != nil {
			return money.ErrUnavailable
		}
		if err := tx.Model(&chargebackRow{}).Where("payment_id = ?", chargeback.PaymentID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&chargedBack).Error; err != nil {
			return money.ErrUnavailable
		}
		if chargeback.AmountMinor > payment.CommissionableAmountMinor-refunded-chargedBack {
			return money.ErrInvalid
		}
		if err := tx.Create(&chargebackRow{ChargebackID: chargeback.ChargebackID, PaymentID: chargeback.PaymentID, AmountMinor: chargeback.AmountMinor, OccurredAt: money.NormalizeTimestamp(chargeback.OccurredAt), ProviderReference: chargeback.ProviderReference}).Error; err != nil {
			return money.ErrUnavailable
		}
		return nil
	})
}

func (r *Repository) RecordRefundSettlementAndNotify(ctx context.Context, refund money.RefundSettlement, observer money.SettlementObserver) error {
	if observer == nil {
		return money.ErrInvalid
	}
	if err := r.RecordRefundSettlement(ctx, refund); err != nil {
		return err
	}
	return observer.ObserveRefundSettlement(ctx, refund)
}

func (r *Repository) RecordChargebackSettlementAndNotify(ctx context.Context, chargeback money.ChargebackSettlement, observer money.SettlementObserver) error {
	if observer == nil {
		return money.ErrInvalid
	}
	if err := r.RecordChargebackSettlement(ctx, chargeback); err != nil {
		return err
	}
	return observer.ObserveChargebackSettlement(ctx, chargeback)
}

func (r *Repository) CreatePayoutMethod(ctx context.Context, method money.PayoutMethod) error {
	if r == nil || r.db == nil || method.Validate() != nil {
		return money.ErrInvalid
	}
	return r.db.WithContext(ctx).Create(&payoutMethodRow{MethodID: method.MethodID, SubjectUserID: method.SubjectUserID, Type: string(method.Type), DisplayName: method.DisplayName, MaskedDestination: method.MaskedDestination, SecureReference: append([]byte(nil), method.SecureReference...), Status: string(method.Status), CreatedAt: method.CreatedAt.UTC(), UpdatedAt: method.UpdatedAt.UTC(), Version: method.Version}).Error
}

func (r *Repository) CreatePayoutMethodIdempotent(ctx context.Context, method money.PayoutMethod, idempotencyKey, fingerprint string) (money.PayoutMethod, error) {
	if r == nil || r.db == nil || method.Validate() != nil || idempotencyKey == "" || fingerprint == "" {
		return money.PayoutMethod{}, money.ErrInvalid
	}
	var out money.PayoutMethod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var operation payoutMethodOperationRow
		if err := tx.Where("idempotency_key = ?", idempotencyKey).Take(&operation).Error; err == nil {
			if operation.Fingerprint != fingerprint {
				return money.ErrConflict
			}
			var row payoutMethodRow
			if err := tx.Where("method_id = ?", operation.MethodID).Take(&row).Error; err != nil {
				return money.ErrUnavailable
			}
			out = payoutMethodFromRow(row)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return money.ErrUnavailable
		}
		row := payoutMethodRow{MethodID: method.MethodID, SubjectUserID: method.SubjectUserID, Type: string(method.Type), DisplayName: method.DisplayName, MaskedDestination: method.MaskedDestination, SecureReference: append([]byte(nil), method.SecureReference...), Status: string(method.Status), CreatedAt: method.CreatedAt.UTC(), UpdatedAt: method.UpdatedAt.UTC(), Version: method.Version}
		if err := tx.Create(&row).Error; err != nil {
			return money.ErrUnavailable
		}
		if err := tx.Create(&payoutMethodOperationRow{IdempotencyKey: idempotencyKey, MethodID: method.MethodID, Fingerprint: fingerprint, CreatedAt: time.Now().UTC()}).Error; err != nil {
			return money.ErrUnavailable
		}
		out = method
		return nil
	})
	return out, err
}

func payoutMethodFromRow(row payoutMethodRow) money.PayoutMethod {
	return money.PayoutMethod{MethodID: row.MethodID, SubjectUserID: row.SubjectUserID, Type: money.PayoutMethodType(row.Type), DisplayName: row.DisplayName, MaskedDestination: row.MaskedDestination, SecureReference: append([]byte(nil), row.SecureReference...), Status: money.PayoutMethodStatus(row.Status), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), Version: row.Version}
}

func (r *Repository) HasValidPayoutMethod(ctx context.Context, subject string, methodID string) (bool, error) {
	if r == nil || r.db == nil || subject == "" {
		return false, money.ErrInvalid
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&payoutMethodRow{}).Where("subject_user_id = ? AND method_id = ? AND status = ?", subject, methodID, string(money.PayoutMethodActive)).Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListActivePayoutMethods(ctx context.Context, subject string) ([]money.PayoutMethodSummary, error) {
	if r == nil || r.db == nil || subject == "" {
		return nil, money.ErrInvalid
	}
	var rows []payoutMethodRow
	if err := r.db.WithContext(ctx).Where("subject_user_id = ? AND status = ?", subject, string(money.PayoutMethodActive)).Order("created_at ASC, method_id ASC").Find(&rows).Error; err != nil {
		return nil, money.ErrUnavailable
	}
	out := make([]money.PayoutMethodSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, money.PayoutMethodSummary{MethodID: row.MethodID, SubjectUserID: row.SubjectUserID, Type: money.PayoutMethodType(row.Type), DisplayName: row.DisplayName, MaskedDestination: row.MaskedDestination, Status: money.PayoutMethodStatus(row.Status), Version: row.Version})
	}
	return out, nil
}
