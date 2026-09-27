package commercialbilling

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type topUpAttemptRow struct {
	OrderID        string `gorm:"primaryKey;size:128"`
	OrganizationID string `gorm:"index;size:128;not null"`
	Provider       string `gorm:"index;size:32;not null"`
	TradeScope     string `gorm:"uniqueIndex;size:64;not null"`
	Fingerprint    string `gorm:"size:64;not null"`
	Phase          string
	Version        int64
	NextCheckAt    time.Time `gorm:"index"`
	NeedsReconcile bool
	Payload        []byte
}

func (topUpAttemptRow) TableName() string { return "commercial_topup_attempts" }

type topUpInboxRow struct {
	Identity    string `gorm:"primaryKey;size:64"`
	TradeScope  string `gorm:"index;size:64;not null"`
	Fingerprint string
	Payload     []byte
	CreatedAt   time.Time
}

func (topUpInboxRow) TableName() string { return "commercial_topup_inbox" }

type topUpRefundRow struct {
	RefundID       string `gorm:"primaryKey;size:128"`
	OrderID        string `gorm:"uniqueIndex:uq_topup_refund_intent,priority:1;size:128;not null"`
	IdempotencyKey string `gorm:"uniqueIndex:uq_topup_refund_intent,priority:2;size:192;not null"`
	Provider       string
	State          string
	Version        int64
	NextCheckAt    time.Time `gorm:"index"`
	Fingerprint    string
	Payload        []byte
}

func (topUpRefundRow) TableName() string { return "commercial_topup_refunds" }
func migrateTopUp(db *gorm.DB) error {
	return db.AutoMigrate(&topUpAttemptRow{}, &topUpInboxRow{}, &topUpRefundRow{})
}

func tradeScope(m billing.TopUpMerchant, trade string) string {
	return money.TopUpFingerprint([]string{string(m.Provider), m.Environment, m.MerchantID, trade})
}
func decodeAttempt(row topUpAttemptRow) (billing.TopUpPaymentAttempt, error) {
	var a billing.TopUpPaymentAttempt
	if json.Unmarshal(row.Payload, &a) != nil {
		return a, billing.ErrConflict
	}
	a.Version = row.Version
	a.NextCheckAt = row.NextCheckAt
	a.NeedsReconcile = row.NeedsReconcile
	if a.Validate() != nil || a.OrderID != row.OrderID || a.OrganizationID != row.OrganizationID || a.Fingerprint() != row.Fingerprint || a.Phase != billing.TopUpPhase(row.Phase) {
		return a, billing.ErrConflict
	}
	return a, nil
}

func (r *Repository) CreateTopUpAttempt(ctx context.Context, req billing.CreateWalletTopUpOrderRequest, merchant billing.TopUpMerchant, now, expiry time.Time) (billing.TopUpPaymentAttempt, error) {
	var out billing.TopUpPaymentAttempt
	if req.Provider != merchant.Provider {
		return out, billing.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id := uuid.NewString()
		now = money.NormalizeTimestamp(now)
		expiry = money.NormalizeTimestamp(expiry)
		a := billing.TopUpPaymentAttempt{AttemptID: uuid.NewString(), OrderID: id, OrganizationID: req.OrganizationID, ActorID: req.ActorID, IdempotencyKey: req.IdempotencyKey, Merchant: merchant, MerchantOrderID: strings.ReplaceAll(uuid.NewString(), "-", ""), Currency: req.Currency, AmountMinor: req.AmountMinor, ExpiresAt: expiry, CreatedAt: now, Phase: billing.TopUpCreated, Version: 1, NextCheckAt: now}
		if a.Validate() != nil {
			return billing.ErrInvalid
		}
		row := orderRow{OrderID: id, OrganizationID: a.OrganizationID, Kind: string(billing.OrderWalletTopUp), Description: "钱包充值", Currency: a.Currency, AmountMinor: a.AmountMinor, Status: string(billing.OrderPending), IdempotencyKey: a.IdempotencyKey, RequestFingerprint: a.Fingerprint(), ActorID: a.ActorID, Version: 1, CreatedAt: now, UpdatedAt: now}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing orderRow
			if err := tx.Where("organization_id = ? AND idempotency_key = ?", req.OrganizationID, req.IdempotencyKey).Take(&existing).Error; err != nil {
				return err
			}
			if existing.Kind != string(billing.OrderWalletTopUp) {
				return billing.ErrConflict
			}
			var stored topUpAttemptRow
			if err := tx.Where("order_id = ?", existing.OrderID).Take(&stored).Error; err != nil {
				return err
			}
			var err error
			out, err = decodeAttempt(stored)
			if err != nil {
				return err
			}
			if out.ActorID != req.ActorID || out.Currency != req.Currency || out.AmountMinor != req.AmountMinor || out.Merchant.Provider != req.Provider {
				return billing.ErrConflict
			}
			return nil
		}
		payload, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if err := tx.Create(&topUpAttemptRow{OrderID: id, OrganizationID: a.OrganizationID, Provider: string(a.Merchant.Provider), TradeScope: tradeScope(a.Merchant, a.MerchantOrderID), Fingerprint: a.Fingerprint(), Phase: string(a.Phase), Version: a.Version, NextCheckAt: now, Payload: payload}).Error; err != nil {
			return err
		}
		out = a
		return nil
	})
	return out, err
}
func (r *Repository) FindTopUpAttempt(ctx context.Context, org, key string) (billing.TopUpPaymentAttempt, error) {
	var row orderRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND idempotency_key = ?", org, key).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return billing.TopUpPaymentAttempt{}, billing.ErrNotFound
	} else if err != nil {
		return billing.TopUpPaymentAttempt{}, err
	}
	if row.Kind != string(billing.OrderWalletTopUp) {
		return billing.TopUpPaymentAttempt{}, billing.ErrConflict
	}
	return r.ReadTopUpAttempt(ctx, org, row.OrderID)
}
func (r *Repository) ReadTopUpAttempt(ctx context.Context, org, order string) (billing.TopUpPaymentAttempt, error) {
	if org == "" || order == "" {
		return billing.TopUpPaymentAttempt{}, billing.ErrInvalid
	}
	var row topUpAttemptRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND order_id = ?", org, order).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return billing.TopUpPaymentAttempt{}, billing.ErrNotFound
	} else if err != nil {
		return billing.TopUpPaymentAttempt{}, err
	}
	return decodeAttempt(row)
}

// Called exclusively after global platform permission, never with a supplied org.
func (r *Repository) ReadTopUpForRefund(ctx context.Context, order string) (billing.TopUpPaymentAttempt, error) {
	var row topUpAttemptRow
	if err := r.db.WithContext(ctx).Where("order_id = ?", order).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return billing.TopUpPaymentAttempt{}, billing.ErrNotFound
	} else if err != nil {
		return billing.TopUpPaymentAttempt{}, err
	}
	return decodeAttempt(row)
}
func saveAttemptTx(tx *gorm.DB, a billing.TopUpPaymentAttempt) (billing.TopUpPaymentAttempt, error) {
	if a.Validate() != nil {
		return a, billing.ErrInvalid
	}
	oldVersion := a.Version
	a.Version++
	payload, err := json.Marshal(a)
	if err != nil {
		return a, err
	}
	result := tx.Model(&topUpAttemptRow{}).Where("order_id = ? AND organization_id = ? AND version = ? AND fingerprint = ?", a.OrderID, a.OrganizationID, oldVersion, a.Fingerprint()).Updates(map[string]any{"payload": payload, "version": a.Version, "phase": string(a.Phase), "next_check_at": a.NextCheckAt, "needs_reconcile": a.NeedsReconcile})
	if result.Error != nil {
		return a, result.Error
	}
	if result.RowsAffected != 1 {
		return a, billing.ErrConflict
	}
	return a, nil
}
func (r *Repository) SaveTopUpAttempt(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.TopUpPaymentAttempt, error) {
	var out billing.TopUpPaymentAttempt
	if a.Phase == billing.TopUpCompleted {
		return out, billing.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = saveAttemptTx(tx, a)
		if err != nil {
			return err
		}
		if a.Phase == billing.TopUpClosedUnpaid {
			result := tx.Model(&orderRow{}).Where("order_id = ? AND organization_id = ? AND status <> ?", a.OrderID, a.OrganizationID, string(billing.OrderFulfilled)).Updates(map[string]any{"status": string(billing.OrderCancelled), "version": gorm.Expr("version + 1"), "updated_at": r.now().UTC()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return billing.ErrConflict
			}
		}
		return nil
	})
	return out, err
}
func (r *Repository) CompleteTopUpOrder(ctx context.Context, a billing.TopUpPaymentAttempt, receipt money.TopUpPostingReceipt) (billing.TopUpPaymentAttempt, error) {
	var out billing.TopUpPaymentAttempt
	if receipt.Validate() != nil || receipt.OrganizationID != a.OrganizationID || receipt.CommercialOrderID != a.OrderID || receipt.OperationID != a.OrderID || receipt.GrossCreditMinor != a.AmountMinor || receipt.Currency != a.Currency || receipt.Binding.Provider != string(a.Merchant.Provider) || receipt.Binding.Environment != a.Merchant.Environment || receipt.Binding.MerchantID != a.Merchant.MerchantID || receipt.Binding.AppID != a.Merchant.AppID {
		return out, billing.ErrConflict
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		a.Phase = billing.TopUpCompleted
		a.PaymentID = receipt.PaymentID
		a.MoneyReceiptID = receipt.ReceiptID
		a.CheckoutCiphertext = nil
		a.CheckoutKind = ""
		a.LeaseToken = ""
		a.LeaseUntil = time.Time{}
		a.NeedsReconcile = false
		if a.ClosedAt != nil && a.LatePaymentCorrectedAt == nil {
			now := money.NormalizeTimestamp(r.now())
			a.LatePaymentCorrectedAt = &now
		}
		var err error
		out, err = saveAttemptTx(tx, a)
		if err != nil {
			return err
		}
		return tx.Model(&orderRow{}).Where("order_id = ? AND organization_id = ? AND kind = ?", a.OrderID, a.OrganizationID, string(billing.OrderWalletTopUp)).Updates(map[string]any{"status": string(billing.OrderFulfilled), "payment_id": receipt.PaymentID, "failure_code": "", "version": gorm.Expr("version + 1"), "updated_at": r.now().UTC()}).Error
	})
	return out, err
}
func (r *Repository) RecordTopUpObservation(ctx context.Context, o billing.ProviderObservation) error {
	if o.Validate() != nil {
		return billing.ErrInvalid
	}
	o.OccurredAt = money.NormalizeTimestamp(o.OccurredAt)
	id := money.TopUpFingerprint([]string{string(o.Merchant.Provider), o.Merchant.Environment, o.Merchant.MerchantID, o.Kind, o.EventID})
	fp := money.TopUpFingerprint(o)
	payload, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := topUpInboxRow{Identity: id, TradeScope: tradeScope(o.Merchant, o.MerchantOrderID), Fingerprint: fp, Payload: payload, CreatedAt: money.NormalizeTimestamp(r.now())}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing topUpInboxRow
			if err := tx.Where("identity = ?", id).Take(&existing).Error; err != nil {
				return err
			}
			if existing.Fingerprint != fp {
				return billing.ErrConflict
			}
			return nil
		}
		// Version advancement invalidates stale recovery/checkout writers and
		// guarantees that an observation arriving during M1 is reconciled again.
		return tx.Model(&topUpAttemptRow{}).Where("trade_scope = ?", row.TradeScope).Updates(map[string]any{"version": gorm.Expr("version + 1"), "needs_reconcile": true, "next_check_at": row.CreatedAt}).Error
	})
}
func (r *Repository) ReadTopUpObservations(ctx context.Context, a billing.TopUpPaymentAttempt) ([]billing.ProviderObservation, error) {
	var rows []topUpInboxRow
	if err := r.db.WithContext(ctx).Where("trade_scope = ?", tradeScope(a.Merchant, a.MerchantOrderID)).Order("created_at, identity").Limit(1001).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 1000 {
		return nil, billing.ErrReconciliationRequired
	}
	out := make([]billing.ProviderObservation, 0, len(rows))
	for _, row := range rows {
		var o billing.ProviderObservation
		if json.Unmarshal(row.Payload, &o) != nil || o.Validate() != nil {
			return nil, billing.ErrConflict
		}
		out = append(out, o)
	}
	return out, nil
}
func (r *Repository) ListRecoverableTopUps(ctx context.Context, provider billing.PaymentProvider, now time.Time, limit int) ([]billing.TopUpPaymentAttempt, error) {
	if limit < 1 || limit > 50 {
		return nil, billing.ErrInvalid
	}
	var rows []topUpAttemptRow
	if err := r.db.WithContext(ctx).Where("provider = ? AND next_check_at <= ? AND (phase NOT IN ? OR needs_reconcile = ?)", provider, now, []string{string(billing.TopUpCompleted), string(billing.TopUpClosedUnpaid)}, true).Order("next_check_at,order_id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]billing.TopUpPaymentAttempt, 0, len(rows))
	for _, row := range rows {
		a, err := decodeAttempt(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func refundFingerprint(in billing.TopUpRefundIntent) string {
	return money.TopUpFingerprint(struct {
		Order, Org, Actor, Key, Reason, Payment string
		Amount, Total                           int64
	}{in.OrderID, in.OrganizationID, in.ActorID, in.IdempotencyKey, in.Reason, in.PaymentID, in.AmountMinor, in.TotalMinor})
}
func (r *Repository) CreateTopUpRefund(ctx context.Context, in billing.TopUpRefundIntent) (billing.TopUpRefundIntent, error) {
	if in.HoldInput().Validate() != nil || in.Merchant.Validate() != nil || len(in.ProviderRequestID) != 32 || in.Version != 1 || in.CreatedAt.IsZero() {
		return in, billing.ErrInvalid
	}
	var out billing.TopUpRefundIntent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		payload, err := json.Marshal(in)
		if err != nil {
			return err
		}
		row := topUpRefundRow{RefundID: in.RefundID, OrderID: in.OrderID, IdempotencyKey: in.IdempotencyKey, Provider: string(in.Merchant.Provider), State: in.State, Version: 1, NextCheckAt: in.NextCheckAt, Fingerprint: refundFingerprint(in), Payload: payload}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			out = in
			return nil
		}
		if err := tx.Where("order_id = ? AND idempotency_key = ?", in.OrderID, in.IdempotencyKey).Take(&row).Error; err != nil {
			return err
		}
		if row.Fingerprint != refundFingerprint(in) || json.Unmarshal(row.Payload, &out) != nil {
			return billing.ErrConflict
		}
		return nil
	})
	return out, err
}
func (r *Repository) SaveTopUpRefund(ctx context.Context, in billing.TopUpRefundIntent) (billing.TopUpRefundIntent, error) {
	old := in.Version
	in.Version++
	payload, err := json.Marshal(in)
	if err != nil {
		return in, err
	}
	result := r.db.WithContext(ctx).Model(&topUpRefundRow{}).Where("refund_id = ? AND version = ? AND fingerprint = ?", in.RefundID, old, refundFingerprint(in)).Updates(map[string]any{"version": in.Version, "state": in.State, "next_check_at": in.NextCheckAt, "payload": payload})
	if result.Error != nil {
		return in, result.Error
	}
	if result.RowsAffected != 1 {
		return in, billing.ErrConflict
	}
	return in, nil
}
func (r *Repository) ListRecoverableTopUpRefunds(ctx context.Context, provider billing.PaymentProvider, now time.Time, limit int) ([]billing.TopUpRefundIntent, error) {
	if limit < 1 || limit > 50 {
		return nil, billing.ErrInvalid
	}
	var rows []topUpRefundRow
	if err := r.db.WithContext(ctx).Where("provider = ? AND next_check_at <= ? AND state NOT IN ?", provider, now, []string{"CONFIRMED", "RELEASED", "REJECTED"}).Order("next_check_at,refund_id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]billing.TopUpRefundIntent, 0, len(rows))
	for _, row := range rows {
		var in billing.TopUpRefundIntent
		if json.Unmarshal(row.Payload, &in) != nil || in.HoldInput().Validate() != nil || in.Version != row.Version || refundFingerprint(in) != row.Fingerprint {
			return nil, billing.ErrConflict
		}
		out = append(out, in)
	}
	return out, nil
}
