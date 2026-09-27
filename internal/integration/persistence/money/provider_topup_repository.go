package money

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	m "task-processor/internal/ledger/money"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// These records add source identity/results to the existing canonical payment,
// settlement and wallet tables. They do not own a second balance.
type providerTopUpRow struct {
	ClaimID           string `gorm:"primaryKey;size:64"`
	PaymentID         string `gorm:"uniqueIndex;size:128;not null"`
	CommercialOrderID string `gorm:"uniqueIndex;size:128;not null"`
	OrganizationID    string `gorm:"size:128;not null"`
	Fingerprint       string `gorm:"size:64;not null"`
	Receipt           []byte
}

func (providerTopUpRow) TableName() string { return "ledger_provider_topup_claims" }

type topUpReversalReceiptRow struct {
	ReceiptID                  string `gorm:"primaryKey;size:128"`
	PaymentID                  string `gorm:"index;size:128;not null"`
	Kind                       string `gorm:"size:16;not null;uniqueIndex:uq_topup_typed_reversal,priority:1"`
	ReversalID                 string `gorm:"size:128;not null;uniqueIndex:uq_topup_typed_reversal,priority:2"`
	OrganizationID             string `gorm:"size:128;not null"`
	CommercialOrderID          string `gorm:"size:128;not null"`
	ProviderAmountMinor        int64
	WalletPrincipalEffectMinor int64
	Fingerprint                string
	Receipt                    []byte
}

func (topUpReversalReceiptRow) TableName() string { return "ledger_topup_reversal_receipts" }

type topUpExcessRow struct {
	ReceiptID                  string `gorm:"primaryKey;size:128"`
	PaymentID                  string
	Kind                       string
	ReversalID                 string
	ProviderReference          string
	ProviderAmountMinor        int64
	WalletPrincipalEffectMinor int64
	ExcessProviderMinor        int64
	State                      string
}

func (topUpExcessRow) TableName() string { return "ledger_topup_excess_reconciliations" }

type topUpRefundHoldRow struct {
	HoldID            string `gorm:"primaryKey;size:128"`
	PaymentID         string `gorm:"index;size:128;not null"`
	ReversalID        string `gorm:"uniqueIndex;size:128;not null"`
	OrganizationID    string
	CommercialOrderID string
	AmountMinor       int64
	ApprovalID        string
	State             string
	Dispatched        bool
	CreatedAt         time.Time
}

func (topUpRefundHoldRow) TableName() string { return "ledger_topup_refund_holds" }

func migrateProviderTopUp(db *gorm.DB) error {
	return db.AutoMigrate(&providerTopUpRow{}, &topUpReversalReceiptRow{}, &topUpExcessRow{}, &topUpRefundHoldRow{})
}

func (r *Repository) AcceptAndPostProviderTopUp(ctx context.Context, in m.ProviderTopUpInput) (m.TopUpPostingReceipt, error) {
	var out m.TopUpPostingReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seed := providerTopUpRow{ClaimID: in.Binding.ClaimID(), PaymentID: in.Payment.PaymentID, CommercialOrderID: in.CommercialOrderID, OrganizationID: in.OrganizationID, Fingerprint: in.Fingerprint()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		var claim providerTopUpRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("claim_id = ?", seed.ClaimID).Take(&claim).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return m.ErrConflict
		} else if err != nil {
			return err
		}
		if claim.Fingerprint != seed.Fingerprint || claim.PaymentID != seed.PaymentID || claim.CommercialOrderID != seed.CommercialOrderID || claim.OrganizationID != seed.OrganizationID {
			return m.ErrConflict
		}
		if len(claim.Receipt) > 0 {
			if err := json.Unmarshal(claim.Receipt, &out); err != nil || out.Validate() != nil {
				return m.ErrConflict
			}
			for _, reversal := range in.KnownReversals {
				if _, err := acceptTopUpReversalTx(tx, reversal); err != nil {
					return err
				}
			}
			return nil
		}
		p := paymentRowFromFact(in.Payment)
		if result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return m.ErrConflict
		}
		wallet, err := lockOrCreateWallet(tx, in.OrganizationID, in.Currency)
		if err != nil {
			return err
		}
		available, debt, err := addTopUp(wallet.AvailableMinor, wallet.DebtMinor, in.AmountMinor)
		if err != nil {
			return err
		}
		if wallet.LifetimeTopUpMinor > math.MaxInt64-in.AmountMinor {
			return m.ErrInvalid
		}
		out = m.TopUpPostingReceipt{ReceiptID: "topup-posting:" + seed.ClaimID, OperationID: in.OperationID, PaymentID: in.Payment.PaymentID, OrganizationID: in.OrganizationID, CommercialOrderID: in.CommercialOrderID, Binding: in.Binding, Currency: in.Currency, GrossCreditMinor: in.AmountMinor, AvailableAddedMinor: available - wallet.AvailableMinor, DebtRepaidMinor: wallet.DebtMinor - debt, RequestFingerprint: seed.Fingerprint, PostedAt: nextWalletProcessingTimestamp(wallet.UpdatedAt)}
		wallet.LifetimeTopUpMinor += in.AmountMinor
		if out.DebtRepaidMinor > 0 {
			wallet.DebtMinor = debt
			id, err := postTopUpEntry(tx, &wallet, m.WalletEntryDebtRepayment, 0, 0, -out.DebtRepaidMinor, in.CommercialOrderID, p.PaymentID, out.ReceiptID+":debt")
			if err != nil {
				return err
			}
			out.CreditEntryIDs = append(out.CreditEntryIDs, id)
		}
		if out.AvailableAddedMinor > 0 {
			wallet.AvailableMinor = available
			id, err := postTopUpEntry(tx, &wallet, m.WalletEntryTopUpCredit, out.AvailableAddedMinor, 0, 0, in.CommercialOrderID, p.PaymentID, out.ReceiptID+":credit")
			if err != nil {
				return err
			}
			out.CreditEntryIDs = append(out.CreditEntryIDs, id)
		}
		binding := walletTopUpSettlementRow{PaymentID: p.PaymentID, CommercialOrderID: in.CommercialOrderID, OrganizationID: in.OrganizationID, Currency: in.Currency, AmountMinor: in.AmountMinor, SettledAt: p.SettledAt, ProviderReference: p.ProviderReference, Version: p.Version}
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		// All known reversals settle inside M1 before spendable funds are visible.
		for _, reversal := range in.KnownReversals {
			receipt, err := acceptTopUpReversalTx(tx, reversal)
			if err != nil {
				return err
			}
			out.KnownReversalReceiptIDs = append(out.KnownReversalReceiptIDs, receipt.ReceiptID)
		}
		out.ResultFingerprint = out.Fingerprint()
		if out.Validate() != nil {
			return m.ErrInvalid
		}
		payload, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Model(&providerTopUpRow{}).Where("claim_id = ?", seed.ClaimID).Update("receipt", payload).Error
	})
	return out, err
}

func paymentRowFromFact(p m.PaymentSettlement) paymentRow {
	return paymentRow{PaymentID: p.PaymentID, PaymentPurpose: p.PaymentPurpose, CommissionTreatment: p.CommissionTreatment, PayerBinding: p.PayerBinding, PayerUserID: p.PayerUserID, Currency: p.Currency, GrossAmountMinor: p.GrossAmountMinor, DiscountAmountMinor: p.DiscountAmountMinor, CommissionableAmountMinor: p.CommissionableAmountMinor, Status: string(p.Status), SettledAt: m.NormalizeTimestamp(p.SettledAt), ProviderReference: p.ProviderReference, Version: p.Version}
}

func (r *Repository) recordProviderReversal(ctx context.Context, payment, id string, kind m.WalletReversalKind, amount int64, at time.Time, reference string) (bool, error) {
	var p paymentRow
	if err := r.db.WithContext(ctx).Where("payment_id = ?", payment).Take(&p).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, m.ErrNotFound
	} else if err != nil {
		return false, err
	}
	if p.PaymentPurpose != m.PaymentPurposeWalletTopUp {
		return false, nil
	}
	var binding walletTopUpSettlementRow
	if err := r.db.WithContext(ctx).Where("payment_id = ?", payment).Take(&binding).Error; err != nil {
		return true, err
	}
	_, err := r.AcceptProviderTopUpReversal(ctx, m.OrganizationWalletReversal{ReversalID: id, PaymentID: payment, CommercialOrderID: binding.CommercialOrderID, OrganizationID: binding.OrganizationID, Currency: binding.Currency, Kind: kind, AmountMinor: amount, OccurredAt: at, ProviderReference: reference})
	return true, err
}

func (r *Repository) ReadTopUpPosting(ctx context.Context, org, order, payment string) (m.TopUpPostingReceipt, error) {
	var out m.TopUpPostingReceipt
	if r == nil || r.db == nil {
		return out, m.ErrUnavailable
	}
	if org == "" || order == "" || payment == "" {
		return out, m.ErrInvalid
	}
	var row providerTopUpRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND commercial_order_id = ? AND payment_id = ?", org, order, payment).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return out, m.ErrNotFound
	} else if err != nil {
		return out, err
	}
	if json.Unmarshal(row.Receipt, &out) != nil || out.Validate() != nil || out.OrganizationID != org || out.CommercialOrderID != order || out.PaymentID != payment || out.RequestFingerprint != row.Fingerprint {
		return m.TopUpPostingReceipt{}, m.ErrConflict
	}
	return out, nil
}

func (r *Repository) AcceptProviderTopUpReversal(ctx context.Context, in m.OrganizationWalletReversal) (m.TopUpReversalReceipt, error) {
	var out m.TopUpReversalReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; out, err = acceptTopUpReversalTx(tx, in); return err })
	return out, err
}

func lockProviderTopUp(tx *gorm.DB, payment string) (walletTopUpSettlementRow, error) {
	var claim providerTopUpRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_id = ?", payment).Take(&claim).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return walletTopUpSettlementRow{}, m.ErrNotFound
	} else if err != nil {
		return walletTopUpSettlementRow{}, err
	}
	var p paymentRow
	// The source claim above is the single per-payment lock. The immutable
	// settlement itself needs no UPDATE privilege, including for row locking.
	if err := tx.Where("payment_id = ?", payment).Take(&p).Error; err != nil {
		return walletTopUpSettlementRow{}, err
	}
	if p.PaymentPurpose != m.PaymentPurposeWalletTopUp {
		return walletTopUpSettlementRow{}, m.ErrInvalid
	}
	var binding walletTopUpSettlementRow
	if err := tx.Where("payment_id = ?", payment).Take(&binding).Error; err != nil {
		return binding, err
	}
	if binding.AmountMinor != p.GrossAmountMinor || binding.OrganizationID != claim.OrganizationID || binding.CommercialOrderID != claim.CommercialOrderID {
		return binding, m.ErrConflict
	}
	return binding, nil
}

func acceptTopUpReversalTx(tx *gorm.DB, in m.OrganizationWalletReversal) (m.TopUpReversalReceipt, error) {
	var out m.TopUpReversalReceipt
	if in.Validate() != nil {
		return out, m.ErrInvalid
	}
	in.OccurredAt = m.NormalizeTimestamp(in.OccurredAt)
	key := m.TopUpReversalKey{PaymentID: in.PaymentID, Kind: in.Kind, ReversalID: in.ReversalID}
	id := key.StorageID()
	fp := m.TopUpFingerprint(in)
	binding, err := lockProviderTopUp(tx, in.PaymentID)
	if err != nil {
		return out, err
	}
	if binding.OrganizationID != in.OrganizationID || binding.CommercialOrderID != in.CommercialOrderID || binding.Currency != in.Currency {
		return out, m.ErrConflict
	}
	var previous topUpReversalReceiptRow
	if err := tx.Where("receipt_id = ?", id).Take(&previous).Error; err == nil {
		if previous.Fingerprint != fp || json.Unmarshal(previous.Receipt, &out) != nil || out.Validate() != nil {
			return out, m.ErrConflict
		}
		return out, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	// The accepted per-kind fact retains its full amount, including excess.
	if in.Kind == m.WalletReversalRefund {
		fact := refundRow{RefundID: in.ReversalID, PaymentID: in.PaymentID, AmountMinor: in.AmountMinor, OccurredAt: in.OccurredAt, ProviderReference: in.ProviderReference}
		if result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fact); result.Error != nil {
			return out, result.Error
		} else if result.RowsAffected != 1 {
			return out, m.ErrConflict
		}
	} else {
		fact := chargebackRow{ChargebackID: in.ReversalID, PaymentID: in.PaymentID, AmountMinor: in.AmountMinor, OccurredAt: in.OccurredAt, ProviderReference: in.ProviderReference}
		if result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fact); result.Error != nil {
			return out, result.Error
		} else if result.RowsAffected != 1 {
			return out, m.ErrConflict
		}
	}
	var totals struct {
		Principal int64
		Provider  int64
	}
	if err := tx.Model(&topUpReversalReceiptRow{}).Where("payment_id = ?", in.PaymentID).Select("COALESCE(SUM(wallet_principal_effect_minor),0) AS principal, COALESCE(SUM(provider_amount_minor),0) AS provider").Scan(&totals).Error; err != nil {
		return out, err
	}
	if totals.Principal < 0 || totals.Principal > binding.AmountMinor || totals.Provider > math.MaxInt64-in.AmountMinor {
		return out, m.ErrInvalid
	}
	d := minInt64(in.AmountMinor, binding.AmountMinor-totals.Principal)
	e := in.AmountMinor - d
	var hold topUpRefundHoldRow
	hasHold := false
	if in.Kind == m.WalletReversalRefund {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hold_id = ?", id).Take(&hold).Error
		if err == nil {
			hasHold = true
			if hold.PaymentID != in.PaymentID || hold.OrganizationID != in.OrganizationID || hold.CommercialOrderID != in.CommercialOrderID || hold.AmountMinor != in.AmountMinor || hold.State == "CONFIRMED" {
				return out, m.ErrConflict
			}
			if hold.State == "RELEASED" {
				hasHold = false
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return out, err
		}
	}
	wallet, err := lockOrCreateWallet(tx, in.OrganizationID, in.Currency)
	if err != nil {
		return out, err
	}
	out = m.TopUpReversalReceipt{ReceiptID: id, Key: key, OrganizationID: in.OrganizationID, CommercialOrderID: in.CommercialOrderID, Currency: in.Currency, ProviderAmountMinor: in.AmountMinor, WalletPrincipalEffectMinor: d, ExcessProviderMinor: e, RequestFingerprint: fp, PostedAt: nextWalletProcessingTimestamp(wallet.UpdatedAt)}
	if hasHold {
		if wallet.ReservedMinor < in.AmountMinor {
			return out, m.ErrConflict
		}
		wallet.ReservedMinor -= in.AmountMinor
		repaid := minInt64(wallet.DebtMinor, e)
		availableAdded := e - repaid
		if wallet.AvailableMinor > math.MaxInt64-availableAdded {
			return out, m.ErrInvalid
		}
		wallet.DebtMinor -= repaid
		wallet.AvailableMinor += availableAdded
		out.HoldID = id
		out.HoldState = "CONFIRMED"
		out.HoldConsumedMinor = d
		out.HoldReleasedMinor = e
		out.HoldDebtRepaidMinor = repaid
		entry, err := postTopUpEntry(tx, &wallet, m.WalletEntryRefundConfirm, availableAdded, -in.AmountMinor, -repaid, in.CommercialOrderID, in.PaymentID, id+":confirm")
		if err != nil {
			return out, err
		}
		out.EntryIDs = append(out.EntryIDs, entry)
		if err := tx.Model(&topUpRefundHoldRow{}).Where("hold_id = ?", id).Update("state", "CONFIRMED").Error; err != nil {
			return out, err
		}
	} else if d > 0 {
		loss := minInt64(wallet.AvailableMinor, d)
		debt := d - loss
		if wallet.DebtMinor > math.MaxInt64-debt {
			return out, m.ErrInvalid
		}
		wallet.AvailableMinor -= loss
		wallet.DebtMinor += debt
		kind := m.WalletEntryRefundReversal
		if in.Kind == m.WalletReversalChargeback {
			kind = m.WalletEntryChargebackReversal
		}
		entry, err := postTopUpEntry(tx, &wallet, kind, -loss, 0, debt, in.CommercialOrderID, in.PaymentID, id)
		if err != nil {
			return out, err
		}
		out.EntryIDs = append(out.EntryIDs, entry)
	}
	if d > 0 {
		if err := tx.Create(&walletReversalRow{ReversalID: id, PaymentID: in.PaymentID, CommercialOrderID: in.CommercialOrderID, OrganizationID: in.OrganizationID, Kind: string(in.Kind), AmountMinor: d, ProviderReference: in.ProviderReference, OccurredAt: in.OccurredAt}).Error; err != nil {
			return out, err
		}
	}
	if e > 0 {
		out.ExcessRecordID = id
		if err := tx.Create(&topUpExcessRow{ReceiptID: id, PaymentID: in.PaymentID, Kind: string(in.Kind), ReversalID: in.ReversalID, ProviderReference: in.ProviderReference, ProviderAmountMinor: in.AmountMinor, WalletPrincipalEffectMinor: d, ExcessProviderMinor: e, State: "OPEN"}).Error; err != nil {
			return out, err
		}
	}
	out.ResultFingerprint = out.Fingerprint()
	if out.Validate() != nil {
		return out, m.ErrInvalid
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	err = tx.Create(&topUpReversalReceiptRow{ReceiptID: id, PaymentID: in.PaymentID, Kind: string(in.Kind), ReversalID: in.ReversalID, OrganizationID: in.OrganizationID, CommercialOrderID: in.CommercialOrderID, ProviderAmountMinor: in.AmountMinor, WalletPrincipalEffectMinor: d, Fingerprint: fp, Receipt: payload}).Error
	return out, err
}

func (r *Repository) ReadTopUpReversal(ctx context.Context, org, order string, key m.TopUpReversalKey) (m.TopUpReversalReceipt, error) {
	var out m.TopUpReversalReceipt
	if r == nil || r.db == nil {
		return out, m.ErrUnavailable
	}
	if org == "" || order == "" || key.Validate() != nil {
		return out, m.ErrInvalid
	}
	var row topUpReversalReceiptRow
	if err := r.db.WithContext(ctx).Where("receipt_id = ? AND organization_id = ? AND commercial_order_id = ?", key.StorageID(), org, order).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return out, m.ErrNotFound
	} else if err != nil {
		return out, err
	}
	if json.Unmarshal(row.Receipt, &out) != nil || out.Validate() != nil || out.Key != key || out.OrganizationID != org || out.CommercialOrderID != order || out.RequestFingerprint != row.Fingerprint {
		return m.TopUpReversalReceipt{}, m.ErrConflict
	}
	return out, nil
}

func (r *Repository) PrepareTopUpRefund(ctx context.Context, in m.TopUpRefundInput) (m.TopUpRefundHold, error) {
	var out m.TopUpRefundHold
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		binding, err := lockProviderTopUp(tx, in.Key.PaymentID)
		if err != nil {
			return err
		}
		if binding.OrganizationID != in.OrganizationID || binding.CommercialOrderID != in.CommercialOrderID {
			return m.ErrConflict
		}
		var existing topUpRefundHoldRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hold_id = ?", in.Key.StorageID()).Take(&existing).Error; err == nil {
			out = refundHold(existing)
			if out.Input != in {
				return m.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var settled int64
		if err := tx.Model(&refundRow{}).Where("refund_id = ?", in.Key.ReversalID).Count(&settled).Error; err != nil {
			return err
		}
		if settled > 0 {
			return m.ErrConflict
		}
		if err := checkRefundBudget(tx, binding, in.AmountMinor); err != nil {
			return err
		}
		wallet, err := lockOrCreateWallet(tx, in.OrganizationID, binding.Currency)
		if err != nil {
			return err
		}
		if wallet.DebtMinor > 0 || wallet.AvailableMinor < in.AmountMinor {
			return m.ErrWalletInsufficientBalance
		}
		if wallet.ReservedMinor > math.MaxInt64-in.AmountMinor {
			return m.ErrInvalid
		}
		wallet.AvailableMinor -= in.AmountMinor
		wallet.ReservedMinor += in.AmountMinor
		row := topUpRefundHoldRow{HoldID: in.Key.StorageID(), PaymentID: in.Key.PaymentID, ReversalID: in.Key.ReversalID, OrganizationID: in.OrganizationID, CommercialOrderID: in.CommercialOrderID, AmountMinor: in.AmountMinor, ApprovalID: in.ApprovalID, State: "RESERVED", CreatedAt: nextWalletProcessingTimestamp(wallet.UpdatedAt)}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if _, err := postTopUpEntry(tx, &wallet, m.WalletEntryRefundReserve, -in.AmountMinor, in.AmountMinor, 0, in.CommercialOrderID, in.Key.PaymentID, row.HoldID+":reserve"); err != nil {
			return err
		}
		out = refundHold(row)
		return nil
	})
	return out, err
}

func checkRefundBudget(tx *gorm.DB, binding walletTopUpSettlementRow, extra int64) error {
	var confirmed, held int64
	if err := tx.Model(&topUpReversalReceiptRow{}).Where("payment_id = ?", binding.PaymentID).Select("COALESCE(SUM(provider_amount_minor),0)").Scan(&confirmed).Error; err != nil {
		return err
	}
	if err := tx.Model(&topUpRefundHoldRow{}).Where("payment_id = ? AND state = ?", binding.PaymentID, "RESERVED").Select("COALESCE(SUM(amount_minor),0)").Scan(&held).Error; err != nil {
		return err
	}
	if confirmed < 0 || held < 0 || extra < 0 || confirmed > binding.AmountMinor || held > binding.AmountMinor-confirmed || extra > binding.AmountMinor-confirmed-held {
		return m.ErrWalletInsufficientBalance
	}
	return nil
}

func (r *Repository) AdmitTopUpRefund(ctx context.Context, in m.TopUpRefundInput) (m.TopUpRefundHold, error) {
	var out m.TopUpRefundHold
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		binding, err := lockProviderTopUp(tx, in.Key.PaymentID)
		if err != nil {
			return err
		}
		var row topUpRefundHoldRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hold_id = ?", in.Key.StorageID()).Take(&row).Error; err != nil {
			return err
		}
		out = refundHold(row)
		if out.Input != in {
			return m.ErrConflict
		}
		if row.Dispatched {
			return nil
		}
		if row.State != "RESERVED" {
			return m.ErrConflict
		}
		if err := checkRefundBudget(tx, binding, 0); err != nil {
			return err
		}
		if err := tx.Model(&topUpRefundHoldRow{}).Where("hold_id = ?", row.HoldID).Update("dispatched", true).Error; err != nil {
			return err
		}
		out.Dispatched = true
		return nil
	})
	return out, err
}

// terminalNotExecuted is only supplied by trusted billing after verification of
// the original provider result. Unknown/timeout must never set it.
func (r *Repository) ReleaseTopUpRefundHold(ctx context.Context, in m.TopUpRefundInput, terminalNotExecuted bool) (m.TopUpRefundHold, error) {
	var out m.TopUpRefundHold
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		binding, err := lockProviderTopUp(tx, in.Key.PaymentID)
		if err != nil {
			return err
		}
		var row topUpRefundHoldRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hold_id = ?", in.Key.StorageID()).Take(&row).Error; err != nil {
			return err
		}
		out = refundHold(row)
		if out.Input != in || row.State == "CONFIRMED" {
			return m.ErrConflict
		}
		if row.State == "RELEASED" {
			return nil
		}
		if row.Dispatched && !terminalNotExecuted {
			return m.ErrConflict
		}
		wallet, err := lockOrCreateWallet(tx, in.OrganizationID, binding.Currency)
		if err != nil {
			return err
		}
		if wallet.ReservedMinor < in.AmountMinor {
			return m.ErrConflict
		}
		available, debt, err := addTopUp(wallet.AvailableMinor, wallet.DebtMinor, in.AmountMinor)
		if err != nil {
			return err
		}
		added, repaid := available-wallet.AvailableMinor, wallet.DebtMinor-debt
		wallet.AvailableMinor = available
		wallet.DebtMinor = debt
		wallet.ReservedMinor -= in.AmountMinor
		if _, err := postTopUpEntry(tx, &wallet, m.WalletEntryRefundRelease, added, -in.AmountMinor, -repaid, in.CommercialOrderID, in.Key.PaymentID, row.HoldID+":release"); err != nil {
			return err
		}
		if err := tx.Model(&topUpRefundHoldRow{}).Where("hold_id = ?", row.HoldID).Update("state", "RELEASED").Error; err != nil {
			return err
		}
		out.State = "RELEASED"
		return nil
	})
	return out, err
}

func refundHold(row topUpRefundHoldRow) m.TopUpRefundHold {
	return m.TopUpRefundHold{HoldID: row.HoldID, Input: m.TopUpRefundInput{Key: m.TopUpReversalKey{PaymentID: row.PaymentID, Kind: m.WalletReversalRefund, ReversalID: row.ReversalID}, OrganizationID: row.OrganizationID, CommercialOrderID: row.CommercialOrderID, AmountMinor: row.AmountMinor, ApprovalID: row.ApprovalID}, State: row.State, Dispatched: row.Dispatched, CreatedAt: row.CreatedAt.UTC()}
}

func postTopUpEntry(tx *gorm.DB, wallet *organizationWalletRow, kind m.WalletEntryKind, available, reserved, debt int64, order, payment, source string) (string, error) {
	wallet.Version++
	wallet.UpdatedAt = nextWalletProcessingTimestamp(wallet.UpdatedAt)
	row := organizationWalletEntryRow{EntryID: uuid.NewString(), OrganizationID: wallet.OrganizationID, Currency: wallet.Currency, Kind: string(kind), AvailableDelta: available, ReservedDelta: reserved, DebtDelta: debt, AvailableAfter: wallet.AvailableMinor, ReservedAfter: wallet.ReservedMinor, DebtAfter: wallet.DebtMinor, CommercialOrderID: order, PaymentID: payment, SourceIdentity: source, OccurredAt: wallet.UpdatedAt}
	if walletEntry(row).Validate() != nil || walletSnapshot(*wallet).Validate() != nil {
		return "", m.ErrInvalid
	}
	if err := saveWalletRow(tx, *wallet); err != nil {
		return "", err
	}
	return row.EntryID, tx.Create(&row).Error
}
