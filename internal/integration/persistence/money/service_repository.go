package money

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	m "task-processor/internal/ledger/money"
	"time"
)

// A shared original-channel claim prevents the same transaction being accepted
// as both a top-up and a service payment in a fresh installation.
type channelPaymentClaimRow struct {
	ClaimID   string `gorm:"primaryKey;size:64"`
	PaymentID string `gorm:"uniqueIndex;size:128;not null"`
	Purpose   string `gorm:"size:32;not null"`
}

func (channelPaymentClaimRow) TableName() string { return "ledger_channel_payment_claims" }

type servicePaymentRow struct {
	ChannelFeeMinor                                                                                                int64
	ChannelFeeObserved                                                                                             bool
	OrderID                                                                                                        string `gorm:"primaryKey;size:128"`
	RequestID                                                                                                      string `gorm:"uniqueIndex;size:128;not null"`
	ClaimID                                                                                                        string `gorm:"uniqueIndex;size:64;not null"`
	PaymentID                                                                                                      string `gorm:"uniqueIndex;size:128;not null"`
	BuyerOrganizationID, ProviderOrganizationID                                                                    string
	Fingerprint                                                                                                    string
	Input, Receipt                                                                                                 []byte
	GrossMinor, RefundedMinor, ChargedBackMinor, SharedMinor, ReturnedMinor, ReleasedMinor, AutomaticReleasedMinor int64
	PendingOperationID, ReconciliationReason                                                                       string
}

func (servicePaymentRow) TableName() string { return "ledger_service_payment_bindings" }

type serviceReservationRow struct {
	OperationID   string `gorm:"primaryKey;size:128"`
	OrderID       string `gorm:"index;size:128;not null"`
	Kind          string
	Fingerprint   string
	Operation     []byte
	State         string
	Dispatched    bool
	DenialProofID string
	Receipt       []byte
}

func (serviceReservationRow) TableName() string { return "ledger_service_operation_reservations" }

type serviceEffectRow struct {
	OperationID string `gorm:"primaryKey;size:128"`
	OrderID     string `gorm:"index;size:128;not null"`
	Kind        string
	Fingerprint string
	Receipt     []byte
}

func (serviceEffectRow) TableName() string { return "ledger_service_effect_receipts" }
func migrateServicePayments(db *gorm.DB) error {
	return db.AutoMigrate(&channelPaymentClaimRow{}, &servicePaymentRow{}, &serviceReservationRow{}, &serviceEffectRow{})
}

// A verified refund state without its original refund facts is not available
// balance. Keep the paid fact and an immutable uncertainty proof together.
func (r *Repository) ObserveServiceRefundUncertainty(ctx context.Context, in m.ServicePaymentInput, proof string) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil || m.ValidateServiceUncertaintyProof(proof) != nil {
		return out, m.ErrInvalid
	}
	fp := m.ServiceFingerprint(struct{ Payment, Proof string }{in.Fingerprint(), proof})
	identity := "service-uncertainty:" + fp
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.OrderID)
		if err != nil {
			return err
		}
		if row.Fingerprint != in.Fingerprint() || len(row.Receipt) == 0 {
			return m.ErrConflict
		}
		var prior serviceEffectRow
		if err := tx.Where("operation_id=?", identity).Take(&prior).Error; err == nil {
			if prior.Fingerprint != fp {
				return m.ErrConflict
			}
			return decodeServiceReceipt(prior.Receipt, &out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		out = m.ServiceReceipt{ReceiptID: identity, OrderID: in.OrderID, OperationID: identity, Kind: "REFUND_UNCERTAINTY", RequestFingerprint: in.Fingerprint(), ProviderReference: proof, OccurredAt: m.NormalizeTimestamp(in.Payment.SettledAt)}
		out.ResultFingerprint = out.Fingerprint()
		payload, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if err := tx.Create(&serviceEffectRow{OperationID: identity, OrderID: in.OrderID, Kind: string(out.Kind), Fingerprint: fp, Receipt: payload}).Error; err != nil {
			return err
		}
		if row.ReconciliationReason == "" {
			return tx.Model(&row).Update("reconciliation_reason", "CHANNEL_REFUND_REQUIRES_RECONCILIATION").Error
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) ResolveFailedServiceOperation(ctx context.Context, in m.ServiceOperationFailure) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	in.OccurredAt = m.NormalizeTimestamp(in.OccurredAt)
	identity := "service-failure:" + in.Operation.OperationID
	fp := m.ServiceFingerprint(in)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.Operation.OrderID)
		if err != nil {
			return err
		}
		var prior serviceEffectRow
		if err := tx.Where("operation_id=?", identity).Take(&prior).Error; err == nil {
			if prior.Fingerprint != fp {
				return m.ErrConflict
			}
			return decodeServiceReceipt(prior.Receipt, &out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var reservation serviceReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("operation_id=?", in.Operation.OperationID).Take(&reservation).Error; err != nil {
			return m.ErrNotFound
		}
		if reservation.Fingerprint != m.ServiceFingerprint(in.Operation) || !reservation.Dispatched || reservation.State != "PREPARED" || row.PendingOperationID != in.Operation.OperationID {
			return m.ErrConflict
		}
		var successful int64
		if err := tx.Model(&serviceEffectRow{}).Where("operation_id=?", in.Operation.OperationID).Count(&successful).Error; err != nil {
			return err
		}
		if successful != 0 {
			return m.ErrConflict
		}
		out = m.ServiceReceipt{ReceiptID: identity, OrderID: row.OrderID, OperationID: in.Operation.OperationID, Kind: m.ServiceEffectKind("FAILED_" + string(in.Operation.Kind)), AmountMinor: 0, RequestFingerprint: m.ServiceFingerprint(in.Operation), ProviderReference: in.ProviderReference, OccurredAt: in.OccurredAt}
		out.ResultFingerprint = out.Fingerprint()
		payload, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if err := tx.Create(&serviceEffectRow{OperationID: identity, OrderID: row.OrderID, Kind: string(out.Kind), Fingerprint: fp, Receipt: payload}).Error; err != nil {
			return err
		}
		if err := tx.Model(&reservation).Updates(map[string]any{"state": "FAILED", "denial_proof_id": in.ProofID}).Error; err != nil {
			return err
		}
		return tx.Model(&row).Update("pending_operation_id", "").Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func claimChannelPayment(tx *gorm.DB, b m.ProviderPaymentBinding, paymentID, purpose string) error {
	seed := channelPaymentClaimRow{ClaimID: b.ClaimID(), PaymentID: paymentID, Purpose: purpose}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	var got channelPaymentClaimRow
	// INSERT ON CONFLICT waits for the competing claim; the next READ COMMITTED
	// statement reads the winner without UPDATE authority on immutable claims.
	if err := tx.Where("claim_id=?", seed.ClaimID).Take(&got).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return m.ErrConflict
	} else if err != nil {
		return err
	}
	if got.PaymentID != paymentID || got.Purpose != purpose {
		return m.ErrConflict
	}
	return nil
}
func (r *Repository) AcceptServicePayment(ctx context.Context, in m.ServicePaymentInput) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := claimChannelPayment(tx, in.Binding, in.Payment.PaymentID, m.PaymentPurposeServicePurchase); err != nil {
			return err
		}
		encoded, err := json.Marshal(in)
		if err != nil {
			return err
		}
		seed := servicePaymentRow{OrderID: in.OrderID, RequestID: in.RequestID, ClaimID: in.Binding.ClaimID(), PaymentID: in.Payment.PaymentID, BuyerOrganizationID: in.BuyerOrganizationID, ProviderOrganizationID: in.ProviderOrganizationID, Fingerprint: in.Fingerprint(), Input: encoded, GrossMinor: in.Payment.GrossAmountMinor}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		var saved servicePaymentRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=?", in.OrderID).Take(&saved).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return m.ErrConflict
		} else if err != nil {
			return err
		}
		if saved.Fingerprint != seed.Fingerprint || saved.ClaimID != seed.ClaimID {
			return m.ErrConflict
		}
		if len(saved.Receipt) > 0 {
			return decodeServiceReceipt(saved.Receipt, &out)
		}
		payment := paymentRowFromFact(in.Payment)
		if result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&payment); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return m.ErrConflict
		}
		out = m.ServiceReceipt{ReceiptID: "service-payment:" + seed.ClaimID, OrderID: in.OrderID, OperationID: in.OrderID, RequestFingerprint: in.Fingerprint(), AmountMinor: in.Payment.GrossAmountMinor, ProviderReference: in.Payment.ProviderReference, OccurredAt: m.NormalizeTimestamp(in.Payment.SettledAt)}
		out.ResultFingerprint = out.Fingerprint()
		saved.Receipt, err = json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Model(&saved).Update("receipt", saved.Receipt).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func decodeServiceReceipt(data []byte, out *m.ServiceReceipt) error {
	if json.Unmarshal(data, out) != nil || out.Validate() != nil {
		return m.ErrConflict
	}
	return nil
}
func (r *Repository) ReadServicePayment(ctx context.Context, in m.ServicePaymentInput) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	var row servicePaymentRow
	if err := r.db.WithContext(ctx).Where("order_id=?", in.OrderID).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return out, m.ErrNotFound
	} else if err != nil {
		return out, err
	}
	if row.Fingerprint != in.Fingerprint() {
		return out, m.ErrConflict
	}
	err := decodeServiceReceipt(row.Receipt, &out)
	return out, err
}
func serviceAllocationPolicy(row servicePaymentRow) (m.ServiceAllocationPolicy, error) {
	var in m.ServicePaymentInput
	if json.Unmarshal(row.Input, &in) != nil || in.Validate() != nil || in.Fingerprint() != row.Fingerprint || in.OrderID != row.OrderID || in.Payment.GrossAmountMinor != row.GrossMinor {
		return m.ServiceAllocationPolicy{}, m.ErrConflict
	}
	return in.Allocation, nil
}

func serviceFunds(row servicePaymentRow) (m.ServiceFundsView, error) {
	policy, err := serviceAllocationPolicy(row)
	if err != nil {
		return m.ServiceFundsView{}, err
	}
	reversed := row.RefundedMinor
	if row.ChargedBackMinor > row.GrossMinor-reversed {
		reversed = row.GrossMinor
	} else {
		reversed += row.ChargedBackMinor
	}
	p, s, err := m.ServiceAllocation(row.GrossMinor, reversed, policy)
	if err != nil {
		return m.ServiceFundsView{}, err
	}
	return m.ServiceFundsView{Allocation: policy, ChannelFeeMinor: row.ChannelFeeMinor, ChannelFeeObserved: row.ChannelFeeObserved, OrderID: row.OrderID, PaymentID: row.PaymentID, BuyerOrganizationID: row.BuyerOrganizationID, ProviderOrganizationID: row.ProviderOrganizationID, Currency: m.WalletCurrencyCNY, GrossMinor: row.GrossMinor, RefundedMinor: row.RefundedMinor, ChargedBackMinor: row.ChargedBackMinor, PlatformMinor: p, ProviderMinor: s, SharedMinor: row.SharedMinor, ReturnedMinor: row.ReturnedMinor, ReleasedMinor: row.ReleasedMinor, AutomaticReleasedMinor: row.AutomaticReleasedMinor, PendingOperationID: row.PendingOperationID, ReconciliationReason: row.ReconciliationReason}, nil
}
func (r *Repository) ReadServiceFunds(ctx context.Context, orderID string) (m.ServiceFundsView, error) {
	var row servicePaymentRow
	if r == nil || r.db == nil || orderID == "" {
		return m.ServiceFundsView{}, m.ErrInvalid
	}
	if err := r.db.WithContext(ctx).Where("order_id=?", orderID).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return m.ServiceFundsView{}, m.ErrNotFound
	} else if err != nil {
		return m.ServiceFundsView{}, err
	}
	return serviceFunds(row)
}
func lockServicePayment(tx *gorm.DB, orderID string) (servicePaymentRow, error) {
	var row servicePaymentRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=?", orderID).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return row, m.ErrNotFound
	} else if err != nil {
		return row, err
	}
	if len(row.Receipt) == 0 {
		return row, m.ErrConflict
	}
	return row, nil
}
func validateServiceOperation(tx *gorm.DB, row servicePaymentRow, in m.ServiceOperation) error {
	funds, err := serviceFunds(row)
	if err != nil {
		return err
	}
	switch in.Kind {
	case m.ServiceShare:
		if row.ReleasedMinor > 0 || row.AutomaticReleasedMinor > 0 || row.ReturnedMinor > 0 || in.AmountMinor != funds.PlatformMinor-row.SharedMinor {
			return m.ErrConflict
		}
	case m.ServiceFinish:
		if row.ReleasedMinor > 0 || row.AutomaticReleasedMinor > 0 || row.SharedMinor-row.ReturnedMinor != funds.PlatformMinor || in.AmountMinor != funds.ProviderMinor {
			return m.ErrConflict
		}
	case m.ServiceRefundRelease:
		if row.SharedMinor == 0 || row.ReleasedMinor > 0 || row.AutomaticReleasedMinor > 0 || in.AmountMinor != row.GrossMinor-row.RefundedMinor-row.ChargedBackMinor-row.SharedMinor {
			return m.ErrConflict
		}
	case m.ServiceReturn:
		var effect serviceEffectRow
		if err := tx.Where("operation_id=? AND order_id=? AND kind=?", in.OriginalShareID, row.OrderID, string(m.ServiceShare)).Take(&effect).Error; err != nil {
			return m.ErrConflict
		}
		var share m.ServiceReceipt
		if decodeServiceReceipt(effect.Receipt, &share) != nil || in.AmountMinor > share.AmountMinor-row.ReturnedMinor {
			return m.ErrConflict
		}
	case m.ServiceRefund, m.ServiceChargeback:
		available := row.GrossMinor - row.RefundedMinor - row.ChargedBackMinor
		if in.AmountMinor > available {
			return m.ErrInvalid
		}
		target, _, err := m.ServiceAllocation(row.GrossMinor, row.RefundedMinor+row.ChargedBackMinor+in.AmountMinor, funds.Allocation)
		if err != nil {
			return err
		}
		if row.SharedMinor-row.ReturnedMinor > target {
			return m.ErrConflict
		}
	default:
		return m.ErrInvalid
	}
	return nil
}
func (r *Repository) PrepareServiceOperation(ctx context.Context, in m.ServiceOperation) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.OrderID)
		if err != nil {
			return err
		}
		var existing serviceReservationRow
		if err := tx.Where("operation_id=?", in.OperationID).Take(&existing).Error; err == nil {
			if existing.Fingerprint != m.ServiceFingerprint(in) {
				return m.ErrConflict
			}
			if existing.State == "ABANDONED" || existing.State == "FAILED" {
				return m.ErrConflict
			}
			return decodeServiceReceipt(existing.Receipt, &out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.PendingOperationID != "" || row.ReconciliationReason != "" {
			return m.ErrConflict
		}
		if err := validateServiceOperation(tx, row, in); err != nil {
			return err
		}
		out = m.ServiceReceipt{ReceiptID: "service-reservation:" + in.OperationID, OrderID: in.OrderID, OperationID: in.OperationID, Kind: in.Kind, AmountMinor: in.AmountMinor, RequestFingerprint: m.ServiceFingerprint(in), OccurredAt: m.NormalizeTimestamp(time.Now())}
		out.ResultFingerprint = out.Fingerprint()
		operation, err := json.Marshal(in)
		if err != nil {
			return err
		}
		receipt, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if err := tx.Create(&serviceReservationRow{OperationID: in.OperationID, OrderID: in.OrderID, Kind: string(in.Kind), Fingerprint: m.ServiceFingerprint(in), Operation: operation, State: "PREPARED", Receipt: receipt}).Error; err != nil {
			return err
		}
		return tx.Model(&row).Update("pending_operation_id", in.OperationID).Error
	})
	return out, err
}
func (r *Repository) AcceptServiceEffect(ctx context.Context, in m.ServiceEffect) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.Operation.OrderID)
		if err != nil {
			return err
		}
		var existing serviceEffectRow
		if err := tx.Where("operation_id=?", in.Operation.OperationID).Take(&existing).Error; err == nil {
			if existing.Fingerprint != in.Fingerprint() {
				return m.ErrConflict
			}
			return decodeServiceReceipt(existing.Receipt, &out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var reservation serviceReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("operation_id=?", in.Operation.OperationID).Take(&reservation).Error; err != nil {
			return m.ErrConflict
		}
		if reservation.Fingerprint != m.ServiceFingerprint(in.Operation) || reservation.OrderID != row.OrderID || reservation.State != "PREPARED" || row.PendingOperationID != in.Operation.OperationID || in.Operation.AmountMinor > 0 && !reservation.Dispatched {
			return m.ErrConflict
		}
		// A verified involuntary effect cannot erase a previously reserved
		// original operation that really completed at the provider. Preserve
		// both facts and the reconciliation fence, rather than replaying it.
		if row.ReconciliationReason == "" {
			if err := validateServiceOperation(tx, row, in.Operation); err != nil {
				return err
			}
		}
		amount := in.Operation.AmountMinor
		at := m.NormalizeTimestamp(in.OccurredAt)
		switch in.Operation.Kind {
		case m.ServiceShare:
			row.SharedMinor += amount
		case m.ServiceFinish, m.ServiceRefundRelease:
			row.ReleasedMinor += amount
		case m.ServiceReturn:
			row.ReturnedMinor += amount
		case m.ServiceRefund:
			fact := refundRow{RefundID: in.Operation.OperationID, PaymentID: row.PaymentID, AmountMinor: amount, OccurredAt: at, ProviderReference: in.ProviderReference}
			if err := tx.Create(&fact).Error; err != nil {
				return err
			}
			row.RefundedMinor += amount
		case m.ServiceChargeback:
			fact := chargebackRow{ChargebackID: in.Operation.OperationID, PaymentID: row.PaymentID, AmountMinor: amount, OccurredAt: at, ProviderReference: in.ProviderReference}
			if err := tx.Create(&fact).Error; err != nil {
				return err
			}
			row.ChargedBackMinor += amount
		}
		out = m.ServiceReceipt{ReceiptID: "service-effect:" + in.Operation.OperationID, OrderID: row.OrderID, OperationID: in.Operation.OperationID, Kind: in.Operation.Kind, AmountMinor: amount, RequestFingerprint: m.ServiceFingerprint(in.Operation), ProviderReference: in.ProviderReference, OccurredAt: at}
		out.ResultFingerprint = out.Fingerprint()
		receipt, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if err := tx.Create(&serviceEffectRow{OperationID: in.Operation.OperationID, OrderID: row.OrderID, Kind: string(in.Operation.Kind), Fingerprint: in.Fingerprint(), Receipt: receipt}).Error; err != nil {
			return err
		}
		if err := tx.Model(&reservation).Update("state", "CONFIRMED").Error; err != nil {
			return err
		}
		row.PendingOperationID = ""
		return tx.Model(&row).Select("refunded_minor", "charged_back_minor", "shared_minor", "returned_minor", "released_minor", "pending_operation_id").Updates(row).Error
	})
	return out, err
}
func (r *Repository) ReadServiceEffect(ctx context.Context, in m.ServiceOperation) (m.ServiceReceipt, error) {
	var out m.ServiceReceipt
	if r == nil || r.db == nil || in.Validate() != nil {
		return out, m.ErrInvalid
	}
	var row serviceEffectRow
	if err := r.db.WithContext(ctx).Where("operation_id=? AND order_id=? AND kind=?", in.OperationID, in.OrderID, string(in.Kind)).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return out, m.ErrNotFound
	} else if err != nil {
		return out, err
	}
	if err := decodeServiceReceipt(row.Receipt, &out); err != nil {
		return out, err
	}
	if out.RequestFingerprint != m.ServiceFingerprint(in) {
		return m.ServiceReceipt{}, m.ErrConflict
	}
	return out, nil
}

func (r *Repository) AdmitServiceOperation(ctx context.Context, in m.ServiceOperation) error {
	if r == nil || r.db == nil || in.Validate() != nil {
		return m.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.OrderID)
		if err != nil {
			return err
		}
		var reservation serviceReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("operation_id=?", in.OperationID).Take(&reservation).Error; err != nil {
			return m.ErrNotFound
		}
		if reservation.Fingerprint != m.ServiceFingerprint(in) || reservation.State == "ABANDONED" || reservation.State == "FAILED" || row.ReconciliationReason != "" {
			return m.ErrConflict
		}
		if reservation.Dispatched {
			return nil
		}
		if reservation.State != "PREPARED" || row.PendingOperationID != in.OperationID {
			return m.ErrConflict
		}
		return tx.Model(&reservation).Update("dispatched", true).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}

// Only billing's durable proof that source admission was denied before any
// dispatch can abandon a prepared reservation. Timeouts never supply that proof.
func (r *Repository) AbandonUndispatchedServiceOperation(ctx context.Context, in m.ServiceOperation, denialProofID string) error {
	if r == nil || r.db == nil || in.Validate() != nil || denialProofID == "" || len(denialProofID) > 128 {
		return m.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, in.OrderID)
		if err != nil {
			return err
		}
		var reservation serviceReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("operation_id=?", in.OperationID).Take(&reservation).Error; err != nil {
			return m.ErrNotFound
		}
		if reservation.Fingerprint != m.ServiceFingerprint(in) {
			return m.ErrConflict
		}
		if reservation.State == "ABANDONED" {
			if reservation.DenialProofID == denialProofID {
				return nil
			}
			return m.ErrConflict
		}
		if reservation.Dispatched || reservation.State != "PREPARED" || row.PendingOperationID != in.OperationID {
			return m.ErrConflict
		}
		if err := tx.Model(&reservation).Updates(map[string]any{"state": "ABANDONED", "denial_proof_id": denialProofID}).Error; err != nil {
			return err
		}
		return tx.Model(&row).Update("pending_operation_id", "").Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}

// This path consumes a verified involuntary channel fact, rather than inventing
// an approval for a chargeback. A conflict remains in billing's durable inbox.
func (r *Repository) recordServiceChargeback(ctx context.Context, in m.ChargebackSettlement) (bool, error) {
	var payment paymentRow
	if err := r.db.WithContext(ctx).Where("payment_id=? AND payment_purpose=?", in.PaymentID, m.PaymentPurposeServicePurchase).Take(&payment).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var binding servicePaymentRow
	if err := r.db.WithContext(ctx).Where("payment_id=?", payment.PaymentID).Take(&binding).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return true, m.ErrConflict
	} else if err != nil {
		return true, err
	}
	return true, r.ObserveServiceChargeback(ctx, binding.OrderID, in)
}

func (r *Repository) ObserveServiceChargeback(ctx context.Context, orderID string, in m.ChargebackSettlement) error {
	if r == nil || r.db == nil || orderID == "" || in.Validate() != nil {
		return m.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockServicePayment(tx, orderID)
		if err != nil {
			return err
		}
		if row.PaymentID != in.PaymentID {
			return m.ErrConflict
		}
		var old chargebackRow
		if err := tx.Where("chargeback_id=?", in.ChargebackID).Take(&old).Error; err == nil {
			if old.PaymentID != in.PaymentID || old.AmountMinor != in.AmountMinor || old.ProviderReference != in.ProviderReference || !old.OccurredAt.Equal(m.NormalizeTimestamp(in.OccurredAt)) {
				return m.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if in.AmountMinor > row.GrossMinor-row.RefundedMinor-row.ChargedBackMinor {
			return m.ErrConflict
		}
		if err := tx.Create(&chargebackRow{ChargebackID: in.ChargebackID, PaymentID: in.PaymentID, AmountMinor: in.AmountMinor, OccurredAt: m.NormalizeTimestamp(in.OccurredAt), ProviderReference: in.ProviderReference}).Error; err != nil {
			return err
		}
		receipt := m.ServiceReceipt{ReceiptID: "service-chargeback:" + in.ChargebackID, OrderID: orderID, OperationID: in.ChargebackID, Kind: m.ServiceChargeback, AmountMinor: in.AmountMinor, RequestFingerprint: m.ServiceFingerprint(in), ProviderReference: in.ProviderReference, OccurredAt: m.NormalizeTimestamp(in.OccurredAt)}
		receipt.ResultFingerprint = receipt.Fingerprint()
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := tx.Create(&serviceEffectRow{OperationID: receipt.ReceiptID, OrderID: orderID, Kind: string(m.ServiceChargeback), Fingerprint: m.ServiceFingerprint(in), Receipt: encoded}).Error; err != nil {
			return err
		}
		row.ChargedBackMinor += in.AmountMinor
		// Preserve existing reservations and share receipts for exact reconciliation;
		// a later success of the admitted original operation is not a new command.
		return tx.Model(&row).Updates(map[string]any{"charged_back_minor": row.ChargedBackMinor, "reconciliation_reason": "CHANNEL_CHARGEBACK_REQUIRES_RECONCILIATION"}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
