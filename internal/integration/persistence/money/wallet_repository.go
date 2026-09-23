package money

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	ledgermoney "task-processor/internal/ledger/money"
)

type organizationWalletRow struct {
	OrganizationID     string    `gorm:"column:organization_id;primaryKey;size:128"`
	Currency           string    `gorm:"column:currency;primaryKey;size:3"`
	AvailableMinor     int64     `gorm:"column:available_minor;not null;default:0"`
	ReservedMinor      int64     `gorm:"column:reserved_minor;not null;default:0"`
	DebtMinor          int64     `gorm:"column:debt_minor;not null;default:0"`
	LifetimeTopUpMinor int64     `gorm:"column:lifetime_topup_minor;not null;default:0"`
	LifetimeSpendMinor int64     `gorm:"column:lifetime_spend_minor;not null;default:0"`
	Version            int64     `gorm:"column:version;not null;default:1"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null"`
}

func (organizationWalletRow) TableName() string { return "ledger_organization_wallets" }

type organizationWalletEntryRow struct {
	EntryID           string    `gorm:"column:entry_id;primaryKey;size:128"`
	OrganizationID    string    `gorm:"column:organization_id;size:128;not null;index"`
	Currency          string    `gorm:"column:currency;size:3;not null"`
	Kind              string    `gorm:"column:entry_kind;size:40;not null"`
	AvailableDelta    int64     `gorm:"column:available_delta;not null"`
	ReservedDelta     int64     `gorm:"column:reserved_delta;not null"`
	DebtDelta         int64     `gorm:"column:debt_delta;not null"`
	AvailableAfter    int64     `gorm:"column:available_after;not null"`
	ReservedAfter     int64     `gorm:"column:reserved_after;not null"`
	DebtAfter         int64     `gorm:"column:debt_after;not null"`
	CommercialOrderID string    `gorm:"column:commercial_order_id;size:128"`
	PaymentID         string    `gorm:"column:payment_id;size:128"`
	SourceIdentity    string    `gorm:"column:source_identity;size:192;not null"`
	OccurredAt        time.Time `gorm:"column:occurred_at;not null;index"`
}

func (organizationWalletEntryRow) TableName() string { return "ledger_organization_wallet_entries" }

type walletReservationRow struct {
	OrganizationID    string    `gorm:"column:organization_id;size:128;not null;index"`
	ReservationID     string    `gorm:"column:reservation_id;primaryKey;size:128"`
	OperationID       string    `gorm:"column:operation_id;uniqueIndex:uq_wallet_reservation_operation,priority:1;size:128;not null"`
	CommercialOrderID string    `gorm:"column:commercial_order_id;uniqueIndex:uq_wallet_reservation_operation,priority:2;size:128;not null"`
	FinishOperationID *string   `gorm:"column:finish_operation_id;uniqueIndex:uq_wallet_reservation_finish_operation;size:128"`
	Currency          string    `gorm:"column:currency;size:3;not null"`
	AmountMinor       int64     `gorm:"column:amount_minor;not null"`
	State             string    `gorm:"column:state;size:16;not null"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null"`
}

func (walletReservationRow) TableName() string { return "ledger_organization_wallet_reservations" }

type walletTopUpSettlementRow struct {
	PaymentID         string    `gorm:"column:payment_id;primaryKey;size:128"`
	CommercialOrderID string    `gorm:"column:commercial_order_id;uniqueIndex;size:128;not null"`
	OrganizationID    string    `gorm:"column:organization_id;size:128;not null"`
	Currency          string    `gorm:"column:currency;size:3;not null"`
	AmountMinor       int64     `gorm:"column:amount_minor;not null"`
	SettledAt         time.Time `gorm:"column:settled_at;not null"`
	ProviderReference string    `gorm:"column:provider_reference;size:192;not null"`
	Version           int64     `gorm:"column:version;not null"`
}

func (walletTopUpSettlementRow) TableName() string { return "ledger_organization_topup_settlements" }

type walletReversalRow struct {
	ReversalID        string    `gorm:"column:reversal_id;primaryKey;size:128"`
	PaymentID         string    `gorm:"column:payment_id;size:128;not null;index"`
	CommercialOrderID string    `gorm:"column:commercial_order_id;size:128;not null"`
	OrganizationID    string    `gorm:"column:organization_id;size:128;not null"`
	Kind              string    `gorm:"column:kind;size:16;not null"`
	AmountMinor       int64     `gorm:"column:amount_minor;not null"`
	ProviderReference string    `gorm:"column:provider_reference;size:192;not null"`
	OccurredAt        time.Time `gorm:"column:occurred_at;not null"`
}

func (walletReversalRow) TableName() string { return "ledger_organization_wallet_reversals" }

func AutoMigrateWallet(db *gorm.DB) error {
	if db == nil {
		return ledgermoney.ErrUnavailable
	}
	return db.AutoMigrate(&organizationWalletRow{}, &organizationWalletEntryRow{}, &walletReservationRow{}, &walletTopUpSettlementRow{}, &walletReversalRow{})
}

func (r *Repository) ReadOrganizationWallet(ctx context.Context, organizationID, currency string) (ledgermoney.OrganizationWalletSnapshot, error) {
	if r == nil || r.db == nil {
		return ledgermoney.OrganizationWalletSnapshot{}, ledgermoney.ErrUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" || currency != ledgermoney.WalletCurrencyCNY {
		return ledgermoney.OrganizationWalletSnapshot{}, ledgermoney.ErrInvalid
	}
	var row organizationWalletRow
	err := r.db.WithContext(ctx).Where("organization_id = ? AND currency = ?", organizationID, currency).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return emptyWallet(organizationID, currency), nil
	}
	if err != nil {
		return ledgermoney.OrganizationWalletSnapshot{}, ledgermoney.ErrUnavailable
	}
	return walletSnapshot(row), nil
}

func (r *Repository) ReadCommercialPurchaseReservation(ctx context.Context, organizationID, orderID, reservationID string) (ledgermoney.WalletReservation, error) {
	if r == nil || r.db == nil {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	orderID = strings.TrimSpace(orderID)
	reservationID = strings.TrimSpace(reservationID)
	if organizationID == "" || orderID == "" || reservationID == "" {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrInvalid
	}
	var row walletReservationRow
	err := r.db.WithContext(ctx).Where("organization_id = ? AND commercial_order_id = ? AND reservation_id = ?", organizationID, orderID, reservationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrWalletReservationConflict
	}
	if err != nil {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrUnavailable
	}
	return reservationFromRow(row), nil
}

func (r *Repository) ListOrganizationWalletEntries(ctx context.Context, organizationID, currency, cursor string, limit int) (ledgermoney.WalletEntryPage, error) {
	if r == nil || r.db == nil {
		return ledgermoney.WalletEntryPage{}, ledgermoney.ErrUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" || currency != ledgermoney.WalletCurrencyCNY || limit < 0 || limit > ledgermoney.MaxOrganizationWalletEntryPageSize {
		return ledgermoney.WalletEntryPage{}, ledgermoney.ErrInvalid
	}
	if limit == 0 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("organization_id = ? AND currency = ?", organizationID, currency).Order("occurred_at DESC, entry_id DESC").Limit(limit + 1)
	if strings.TrimSpace(cursor) != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
		if err != nil {
			return ledgermoney.WalletEntryPage{}, ledgermoney.ErrInvalid
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return ledgermoney.WalletEntryPage{}, ledgermoney.ErrInvalid
		}
		occurredAt, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return ledgermoney.WalletEntryPage{}, ledgermoney.ErrInvalid
		}
		query = query.Where("(occurred_at < ?) OR (occurred_at = ? AND entry_id < ?)", occurredAt.UTC(), occurredAt.UTC(), parts[1])
	}
	var rows []organizationWalletEntryRow
	if err := query.Find(&rows).Error; err != nil {
		return ledgermoney.WalletEntryPage{}, ledgermoney.ErrUnavailable
	}
	page := ledgermoney.WalletEntryPage{Items: make([]ledgermoney.WalletEntry, 0, minInt(len(rows), limit))}
	if len(rows) > limit {
		last := rows[limit-1]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + last.EntryID))
		rows = rows[:limit]
	}
	for _, row := range rows {
		page.Items = append(page.Items, walletEntry(row))
	}
	return page, nil
}

func (r *Repository) CreditSettledTopUp(ctx context.Context, _ string, settlement ledgermoney.OrganizationTopUpSettlement) (ledgermoney.OrganizationWalletSnapshot, error) {
	if r == nil || r.db == nil || settlement.Validate() != nil {
		return ledgermoney.OrganizationWalletSnapshot{}, ledgermoney.ErrInvalid
	}
	var out ledgermoney.OrganizationWalletSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment paymentRow
		if err := tx.Where("payment_id = ?", settlement.PaymentID).Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ledgermoney.ErrNotFound
			}
			return ledgermoney.ErrUnavailable
		}
		if payment.Currency != settlement.Currency || settlement.AmountMinor > payment.GrossAmountMinor {
			return ledgermoney.ErrConflict
		}
		var existing walletTopUpSettlementRow
		if err := tx.Where("payment_id = ?", settlement.PaymentID).Take(&existing).Error; err == nil {
			if !sameTopUp(existing, settlement) {
				return ledgermoney.ErrConflict
			}
			var row organizationWalletRow
			if err := tx.Where("organization_id = ? AND currency = ?", settlement.OrganizationID, settlement.Currency).Take(&row).Error; err != nil {
				return ledgermoney.ErrUnavailable
			}
			out = walletSnapshot(row)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		var byOrder walletTopUpSettlementRow
		if err := tx.Where("commercial_order_id = ?", settlement.CommercialOrderID).Take(&byOrder).Error; err == nil {
			return ledgermoney.ErrConflict
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		row, err := lockOrCreateWallet(tx, settlement.OrganizationID, settlement.Currency)
		if err != nil {
			return err
		}
		available, debt, err := addTopUp(row.AvailableMinor, row.DebtMinor, settlement.AmountMinor)
		if err != nil {
			return err
		}
		if row.LifetimeTopUpMinor > math.MaxInt64-settlement.AmountMinor {
			return ledgermoney.ErrInvalid
		}
		availableBefore, debtBefore := row.AvailableMinor, row.DebtMinor
		now := ledgermoney.NormalizeTimestamp(settlement.SettledAt)
		row.AvailableMinor, row.DebtMinor, row.LifetimeTopUpMinor = available, debt, row.LifetimeTopUpMinor+settlement.AmountMinor
		row.Version++
		row.UpdatedAt = now
		if err := tx.Save(&row).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := tx.Create(&walletTopUpSettlementRow{PaymentID: settlement.PaymentID, CommercialOrderID: settlement.CommercialOrderID, OrganizationID: settlement.OrganizationID, Currency: settlement.Currency, AmountMinor: settlement.AmountMinor, SettledAt: now, ProviderReference: settlement.ProviderReference, Version: settlement.Version}).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		debtRepaid := debtBefore - debt
		availableAdded := available - availableBefore
		if debtRepaid > 0 {
			debtEntry := organizationWalletEntryRow{EntryID: uuid.NewString(), OrganizationID: settlement.OrganizationID, Currency: settlement.Currency, Kind: string(ledgermoney.WalletEntryDebtRepayment), AvailableAfter: availableBefore, ReservedAfter: row.ReservedMinor, DebtDelta: -debtRepaid, DebtAfter: debt, CommercialOrderID: settlement.CommercialOrderID, PaymentID: settlement.PaymentID, SourceIdentity: settlement.PaymentID + ":debt", OccurredAt: now}
			if err := tx.Create(&debtEntry).Error; err != nil {
				return ledgermoney.ErrUnavailable
			}
		}
		if availableAdded > 0 {
			creditEntry := organizationWalletEntryRow{EntryID: uuid.NewString(), OrganizationID: settlement.OrganizationID, Currency: settlement.Currency, Kind: string(ledgermoney.WalletEntryTopUpCredit), AvailableDelta: availableAdded, AvailableAfter: available, ReservedAfter: row.ReservedMinor, DebtAfter: debt, CommercialOrderID: settlement.CommercialOrderID, PaymentID: settlement.PaymentID, SourceIdentity: settlement.PaymentID, OccurredAt: now}
			if err := tx.Create(&creditEntry).Error; err != nil {
				return ledgermoney.ErrUnavailable
			}
		}
		out = walletSnapshot(row)
		return nil
	})
	return out, err
}

func (r *Repository) ApplyTopUpReversal(ctx context.Context, _ string, reversal ledgermoney.OrganizationWalletReversal) (ledgermoney.OrganizationWalletSnapshot, error) {
	if r == nil || r.db == nil || reversal.Validate() != nil {
		return ledgermoney.OrganizationWalletSnapshot{}, ledgermoney.ErrInvalid
	}
	var out ledgermoney.OrganizationWalletSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var settlement walletTopUpSettlementRow
		if err := tx.Where("payment_id = ? AND commercial_order_id = ?", reversal.PaymentID, reversal.CommercialOrderID).Take(&settlement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ledgermoney.ErrNotFound
			}
			return ledgermoney.ErrUnavailable
		}
		if settlement.OrganizationID != reversal.OrganizationID || settlement.Currency != reversal.Currency {
			return ledgermoney.ErrConflict
		}
		var existing walletReversalRow
		if err := tx.Where("reversal_id = ?", reversal.ReversalID).Take(&existing).Error; err == nil {
			if existing.PaymentID != reversal.PaymentID || existing.AmountMinor != reversal.AmountMinor || existing.Kind != string(reversal.Kind) || existing.ProviderReference != reversal.ProviderReference || !existing.OccurredAt.Equal(ledgermoney.NormalizeTimestamp(reversal.OccurredAt)) {
				return ledgermoney.ErrConflict
			}
			var row organizationWalletRow
			if err := tx.Where("organization_id = ? AND currency = ?", reversal.OrganizationID, reversal.Currency).Take(&row).Error; err != nil {
				return ledgermoney.ErrUnavailable
			}
			out = walletSnapshot(row)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		row, err := lockOrCreateWallet(tx, reversal.OrganizationID, reversal.Currency)
		if err != nil {
			return err
		}
		// Every reversal for this settlement locks the same organization wallet
		// before reading the accumulated total, making the wallet row the
		// serialization fence for both the balance and settlement limit.
		var reversed int64
		if err := tx.Model(&walletReversalRow{}).Where("payment_id = ?", reversal.PaymentID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&reversed).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if reversed > settlement.AmountMinor || reversal.AmountMinor > settlement.AmountMinor-reversed {
			return ledgermoney.ErrConflict
		}
		availableLoss := minInt64(row.AvailableMinor, reversal.AmountMinor)
		debtIncrease := reversal.AmountMinor - availableLoss
		if row.DebtMinor > math.MaxInt64-debtIncrease {
			return ledgermoney.ErrInvalid
		}
		row.AvailableMinor -= availableLoss
		row.DebtMinor += debtIncrease
		row.Version++
		row.UpdatedAt = ledgermoney.NormalizeTimestamp(reversal.OccurredAt)
		if err := tx.Save(&row).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := tx.Create(&walletReversalRow{ReversalID: reversal.ReversalID, PaymentID: reversal.PaymentID, CommercialOrderID: reversal.CommercialOrderID, OrganizationID: reversal.OrganizationID, Kind: string(reversal.Kind), AmountMinor: reversal.AmountMinor, ProviderReference: reversal.ProviderReference, OccurredAt: row.UpdatedAt}).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		kind := ledgermoney.WalletEntryRefundReversal
		if reversal.Kind == ledgermoney.WalletReversalChargeback {
			kind = ledgermoney.WalletEntryChargebackReversal
		}
		if err := tx.Create(&organizationWalletEntryRow{EntryID: uuid.NewString(), OrganizationID: reversal.OrganizationID, Currency: reversal.Currency, Kind: string(kind), AvailableDelta: -availableLoss, DebtDelta: debtIncrease, AvailableAfter: row.AvailableMinor, ReservedAfter: row.ReservedMinor, DebtAfter: row.DebtMinor, CommercialOrderID: reversal.CommercialOrderID, PaymentID: reversal.PaymentID, SourceIdentity: reversal.ReversalID, OccurredAt: row.UpdatedAt}).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		out = walletSnapshot(row)
		return nil
	})
	return out, err
}

func (r *Repository) ReserveCommercialPurchase(ctx context.Context, input ledgermoney.ReserveWalletFundsInput) (ledgermoney.WalletReservation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(input.OrganizationID) == "" || strings.TrimSpace(input.OperationID) == "" || strings.TrimSpace(input.CommercialOrderID) == "" || input.Currency != ledgermoney.WalletCurrencyCNY || input.AmountMinor <= 0 {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrInvalid
	}
	var out ledgermoney.WalletReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing walletReservationRow
		if err := tx.Where("organization_id = ? AND operation_id = ? AND commercial_order_id = ?", input.OrganizationID, input.OperationID, input.CommercialOrderID).Take(&existing).Error; err == nil {
			if existing.AmountMinor != input.AmountMinor || existing.Currency != input.Currency {
				return ledgermoney.ErrWalletReservationConflict
			}
			out = reservationFromRow(existing)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		wallet, err := lockOrCreateWallet(tx, input.OrganizationID, input.Currency)
		if err != nil {
			return err
		}
		// A same-key request can pass the initial lookup before another
		// transaction reserves the wallet. Re-read after acquiring the wallet
		// lock so it replays that reservation instead of cancelling the order
		// for insufficient balance.
		if err := tx.Where("organization_id = ? AND operation_id = ? AND commercial_order_id = ?", input.OrganizationID, input.OperationID, input.CommercialOrderID).Take(&existing).Error; err == nil {
			if existing.AmountMinor != input.AmountMinor || existing.Currency != input.Currency {
				return ledgermoney.ErrWalletReservationConflict
			}
			out = reservationFromRow(existing)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		if wallet.AvailableMinor < input.AmountMinor {
			return ledgermoney.ErrWalletInsufficientBalance
		}
		if wallet.ReservedMinor > math.MaxInt64-input.AmountMinor {
			return ledgermoney.ErrInvalid
		}
		now := time.Now().UTC()
		reservation := walletReservationRow{OrganizationID: input.OrganizationID, ReservationID: uuid.NewString(), OperationID: input.OperationID, CommercialOrderID: input.CommercialOrderID, Currency: input.Currency, AmountMinor: input.AmountMinor, State: string(ledgermoney.WalletReservationReserved), CreatedAt: now, UpdatedAt: now}
		wallet.AvailableMinor -= input.AmountMinor
		wallet.ReservedMinor += input.AmountMinor
		wallet.Version++
		wallet.UpdatedAt = now
		if err := tx.Save(&wallet).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := tx.Create(&reservation).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := createWalletEntry(tx, wallet, ledgermoney.WalletEntryPurchaseReserve, -input.AmountMinor, input.AmountMinor, 0, input.CommercialOrderID, "", input.CommercialOrderID, now); err != nil {
			return err
		}
		out = reservationFromRow(reservation)
		return nil
	})
	return out, err
}

func (r *Repository) CommitCommercialPurchase(ctx context.Context, input ledgermoney.CommitWalletReservationInput) (ledgermoney.WalletReservation, error) {
	return r.finishReservation(ctx, input.OperationID, input.OrganizationID, input.CommercialOrderID, input.ReservationID, ledgermoney.WalletReservationCommitted, ledgermoney.WalletEntryPurchaseCommit, "")
}

func (r *Repository) ReleaseCommercialPurchase(ctx context.Context, input ledgermoney.ReleaseWalletReservationInput) (ledgermoney.WalletReservation, error) {
	return r.finishReservation(ctx, input.OperationID, input.OrganizationID, input.CommercialOrderID, input.ReservationID, ledgermoney.WalletReservationReleased, ledgermoney.WalletEntryPurchaseRelease, input.Reason)
}

func (r *Repository) finishReservation(ctx context.Context, operationID, organizationID, orderID, reservationID string, terminal ledgermoney.WalletReservationState, kind ledgermoney.WalletEntryKind, _ string) (ledgermoney.WalletReservation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(operationID) == "" || len(operationID) > 128 || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(orderID) == "" || strings.TrimSpace(reservationID) == "" {
		return ledgermoney.WalletReservation{}, ledgermoney.ErrInvalid
	}
	var out ledgermoney.WalletReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var operationOwner walletReservationRow
		if err := tx.Where("finish_operation_id = ?", operationID).Take(&operationOwner).Error; err == nil {
			if operationOwner.ReservationID != reservationID || operationOwner.OrganizationID != organizationID || operationOwner.CommercialOrderID != orderID || operationOwner.State != string(terminal) {
				return ledgermoney.ErrWalletReservationConflict
			}
			out = reservationFromRow(operationOwner)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ledgermoney.ErrUnavailable
		}
		var reservation walletReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND reservation_id = ?", organizationID, reservationID).Take(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ledgermoney.ErrWalletReservationConflict
			}
			return ledgermoney.ErrUnavailable
		}
		if reservation.CommercialOrderID != orderID {
			return ledgermoney.ErrWalletReservationConflict
		}
		if reservation.State == string(terminal) {
			return ledgermoney.ErrWalletReservationConflict
		}
		if reservation.State != string(ledgermoney.WalletReservationReserved) {
			return ledgermoney.ErrWalletReservationConflict
		}
		wallet, err := lockOrCreateWallet(tx, organizationID, reservation.Currency)
		if err != nil {
			return err
		}
		if wallet.ReservedMinor < reservation.AmountMinor {
			return ledgermoney.ErrWalletReservationConflict
		}
		availableDelta := int64(0)
		debtDelta := int64(0)
		if terminal == ledgermoney.WalletReservationReleased {
			debtRepaid := minInt64(wallet.DebtMinor, reservation.AmountMinor)
			debtDelta = -debtRepaid
			availableDelta = reservation.AmountMinor - debtRepaid
			if wallet.AvailableMinor > math.MaxInt64-availableDelta {
				return ledgermoney.ErrInvalid
			}
			wallet.AvailableMinor += availableDelta
			wallet.DebtMinor -= debtRepaid
		} else {
			if wallet.LifetimeSpendMinor > math.MaxInt64-reservation.AmountMinor {
				return ledgermoney.ErrInvalid
			}
			wallet.LifetimeSpendMinor += reservation.AmountMinor
		}
		wallet.ReservedMinor -= reservation.AmountMinor
		wallet.Version++
		wallet.UpdatedAt = time.Now().UTC()
		reservation.State = string(terminal)
		reservation.FinishOperationID = &operationID
		reservation.UpdatedAt = wallet.UpdatedAt
		if err := tx.Save(&wallet).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := tx.Save(&reservation).Error; err != nil {
			return ledgermoney.ErrUnavailable
		}
		if err := createWalletEntry(tx, wallet, kind, availableDelta, -reservation.AmountMinor, debtDelta, orderID, "", orderID, wallet.UpdatedAt); err != nil {
			return err
		}
		out = reservationFromRow(reservation)
		return nil
	})
	return out, err
}

func lockOrCreateWallet(tx *gorm.DB, organizationID, currency string) (organizationWalletRow, error) {
	seed := organizationWalletRow{OrganizationID: organizationID, Currency: currency, Version: 1, UpdatedAt: time.Now().UTC()}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return organizationWalletRow{}, ledgermoney.ErrUnavailable
	}
	var row organizationWalletRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND currency = ?", organizationID, currency).Take(&row).Error; err != nil {
		return organizationWalletRow{}, ledgermoney.ErrUnavailable
	}
	return row, nil
}

func createWalletEntry(tx *gorm.DB, wallet organizationWalletRow, kind ledgermoney.WalletEntryKind, availableDelta, reservedDelta, debtDelta int64, orderID, paymentID, source string, occurredAt time.Time) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("%w: wallet source identity is required", ledgermoney.ErrInvalid)
	}
	return tx.Create(&organizationWalletEntryRow{EntryID: uuid.NewString(), OrganizationID: wallet.OrganizationID, Currency: wallet.Currency, Kind: string(kind), AvailableDelta: availableDelta, ReservedDelta: reservedDelta, DebtDelta: debtDelta, AvailableAfter: wallet.AvailableMinor, ReservedAfter: wallet.ReservedMinor, DebtAfter: wallet.DebtMinor, CommercialOrderID: orderID, PaymentID: paymentID, SourceIdentity: source, OccurredAt: occurredAt}).Error
}

func addTopUp(available, debt, amount int64) (int64, int64, error) {
	if debt >= amount {
		return available, debt - amount, nil
	}
	net := amount - debt
	if available > math.MaxInt64-net {
		return 0, 0, ledgermoney.ErrInvalid
	}
	return available + net, 0, nil
}

func sameTopUp(row walletTopUpSettlementRow, value ledgermoney.OrganizationTopUpSettlement) bool {
	return row.CommercialOrderID == value.CommercialOrderID && row.OrganizationID == value.OrganizationID && row.Currency == value.Currency && row.AmountMinor == value.AmountMinor && row.ProviderReference == value.ProviderReference && row.Version == value.Version && row.SettledAt.Equal(ledgermoney.NormalizeTimestamp(value.SettledAt))
}

func walletSnapshot(row organizationWalletRow) ledgermoney.OrganizationWalletSnapshot {
	return ledgermoney.OrganizationWalletSnapshot{OrganizationID: row.OrganizationID, Currency: row.Currency, AvailableMinor: row.AvailableMinor, ReservedMinor: row.ReservedMinor, DebtMinor: row.DebtMinor, LifetimeTopUpMinor: row.LifetimeTopUpMinor, LifetimeSpendMinor: row.LifetimeSpendMinor, Version: row.Version, UpdatedAt: row.UpdatedAt.UTC()}
}

func emptyWallet(organizationID, currency string) ledgermoney.OrganizationWalletSnapshot {
	return ledgermoney.OrganizationWalletSnapshot{OrganizationID: organizationID, Currency: currency, Version: 1, UpdatedAt: time.Now().UTC()}
}

func walletEntry(row organizationWalletEntryRow) ledgermoney.WalletEntry {
	return ledgermoney.WalletEntry{EntryID: row.EntryID, OrganizationID: row.OrganizationID, Currency: row.Currency, Kind: ledgermoney.WalletEntryKind(row.Kind), AvailableDelta: row.AvailableDelta, ReservedDelta: row.ReservedDelta, DebtDelta: row.DebtDelta, AvailableAfter: row.AvailableAfter, ReservedAfter: row.ReservedAfter, DebtAfter: row.DebtAfter, CommercialOrderID: row.CommercialOrderID, PaymentID: row.PaymentID, SourceIdentity: row.SourceIdentity, OccurredAt: row.OccurredAt.UTC()}
}

func reservationFromRow(row walletReservationRow) ledgermoney.WalletReservation {
	return ledgermoney.WalletReservation{ReservationID: row.ReservationID, OperationID: row.OperationID, OrganizationID: row.OrganizationID, CommercialOrderID: row.CommercialOrderID, Currency: row.Currency, AmountMinor: row.AmountMinor, State: ledgermoney.WalletReservationState(row.State), Version: 1, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
