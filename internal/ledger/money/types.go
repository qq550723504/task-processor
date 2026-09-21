package money

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid             = errors.New("money owner invalid input")
	ErrConflict            = errors.New("money owner external identity conflict")
	ErrNotFound            = errors.New("money owner fact not found")
	ErrUnavailable         = errors.New("money owner unavailable")
	ErrUnsupportedMutation = errors.New("money owner mutation is not supported")
)

type PaymentStatus string

const PaymentSettled PaymentStatus = "SETTLED"

type PaymentSettlement struct {
	PaymentID                 string
	PayerUserID               string
	Currency                  string
	GrossAmountMinor          int64
	DiscountAmountMinor       int64
	CommissionableAmountMinor int64
	Status                    PaymentStatus
	SettledAt                 time.Time
	ProviderReference         string
	Version                   int64
}

type RefundSettlement struct {
	RefundID          string
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

type ChargebackSettlement struct {
	ChargebackID      string
	PaymentID         string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

// NormalizeTimestamp matches PostgreSQL timestamptz precision. Canonical
// facts must compare the value that persistence can actually round-trip.
func NormalizeTimestamp(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func (p PaymentSettlement) Validate() error {
	if strings.TrimSpace(p.PaymentID) == "" || strings.TrimSpace(p.PayerUserID) == "" || strings.TrimSpace(p.Currency) == "" || p.Status != PaymentSettled || p.SettledAt.IsZero() || p.GrossAmountMinor <= 0 || p.DiscountAmountMinor < 0 || p.CommissionableAmountMinor <= 0 || p.DiscountAmountMinor > p.GrossAmountMinor || p.CommissionableAmountMinor > p.GrossAmountMinor-p.DiscountAmountMinor || strings.TrimSpace(p.ProviderReference) == "" || p.Version < 1 {
		return ErrInvalid
	}
	return nil
}

func (r RefundSettlement) Validate() error {
	if strings.TrimSpace(r.RefundID) == "" || strings.TrimSpace(r.PaymentID) == "" || r.AmountMinor <= 0 || r.OccurredAt.IsZero() || strings.TrimSpace(r.ProviderReference) == "" {
		return ErrInvalid
	}
	return nil
}

func (c ChargebackSettlement) Validate() error {
	if strings.TrimSpace(c.ChargebackID) == "" || strings.TrimSpace(c.PaymentID) == "" || c.AmountMinor <= 0 || c.OccurredAt.IsZero() || strings.TrimSpace(c.ProviderReference) == "" {
		return ErrInvalid
	}
	return nil
}

type PayoutMethodType string

const (
	PayoutAlipay       PayoutMethodType = "ALIPAY"
	PayoutBankTransfer PayoutMethodType = "BANK_TRANSFER"
)

type PayoutMethodStatus string

const (
	PayoutMethodActive   PayoutMethodStatus = "ACTIVE"
	PayoutMethodDisabled PayoutMethodStatus = "DISABLED"
)

type PayoutMethod struct {
	MethodID          string
	SubjectUserID     string
	Type              PayoutMethodType
	DisplayName       string
	MaskedDestination string
	SecureReference   []byte
	Status            PayoutMethodStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
}

// PayoutMethodSummary is the only representation safe for ordinary account
// reads. SecureReference is deliberately absent so callers cannot accidentally
// serialize the encrypted provider credential.
type PayoutMethodSummary struct {
	MethodID          string
	SubjectUserID     string
	Type              PayoutMethodType
	DisplayName       string
	MaskedDestination string
	Status            PayoutMethodStatus
	Version           int64
}

func (p PayoutMethod) Validate() error {
	if strings.TrimSpace(p.MethodID) == "" || strings.TrimSpace(p.SubjectUserID) == "" || (p.Type != PayoutAlipay && p.Type != PayoutBankTransfer) || strings.TrimSpace(p.DisplayName) == "" || strings.TrimSpace(p.MaskedDestination) == "" || len(p.SecureReference) == 0 || (p.Status != PayoutMethodActive && p.Status != PayoutMethodDisabled) || p.Version < 1 {
		return ErrInvalid
	}
	return nil
}

type SettlementStore interface {
	RecordPaymentSettlement(context.Context, PaymentSettlement) error
	RecordRefundSettlement(context.Context, RefundSettlement) error
	RecordChargebackSettlement(context.Context, ChargebackSettlement) error
}

// SettlementObserver is the one-way projection boundary for downstream
// earnings. The observer receives only facts already accepted by this owner;
// it cannot create a payment settlement.
type SettlementObserver interface {
	ObservePaymentSettlement(context.Context, PaymentSettlement) error
	ObserveRefundSettlement(context.Context, RefundSettlement) error
	ObserveChargebackSettlement(context.Context, ChargebackSettlement) error
}
