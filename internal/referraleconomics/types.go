package referraleconomics

import (
	"context"
	"errors"
	"math/big"
	money "task-processor/internal/ledger/money"
	"time"
)

const (
	CurrencyCNY            = "CNY"
	CommissionRateBPS      = int64(1000)
	BPSDenominator         = int64(10000)
	MinimumWithdrawalMinor = int64(10000)
)

var (
	ErrInvalid             = errors.New("referral economics invalid request")
	ErrConflict            = errors.New("referral economics version conflict")
	ErrIdempotencyConflict = errors.New("referral economics idempotency conflict")
	ErrNotEligible         = errors.New("referral economics withdrawal not eligible")
	ErrInsufficient        = errors.New("referral economics insufficient available amount")
	ErrInvalidTransition   = errors.New("referral economics invalid withdrawal transition")
	ErrUnavailable         = errors.New("referral economics unavailable")
)

type SettledPayment struct {
	PaymentID, Issuer, Subject, Currency string
	NetCashMinor                         int64
	SettledAt                            time.Time
}
type Refund struct {
	PaymentID, RefundID string
	AmountMinor         int64
	RefundedAt          time.Time
}
type Earnings struct {
	Referrer, Currency                                           string
	PendingMinor, AvailableMinor, ReservedMinor, AdjustmentMinor int64
	Version                                                      int64
	UpdatedAt                                                    time.Time
}
type WithdrawalMethod string

const (
	MethodAlipay       WithdrawalMethod = "ALIPAY"
	MethodBankTransfer WithdrawalMethod = "BANK_TRANSFER"
)

type WithdrawalStatus string

const (
	WithdrawalRequested WithdrawalStatus = "REQUESTED"
	WithdrawalApproved  WithdrawalStatus = "APPROVED"
	WithdrawalPaid      WithdrawalStatus = "PAID"
	WithdrawalCanceled  WithdrawalStatus = "CANCELED"
	WithdrawalRejected  WithdrawalStatus = "REJECTED"
)

type Withdrawal struct {
	ID, Referrer, Currency, PayoutReference, PayoutMethodID string
	Method                                                  WithdrawalMethod
	AmountMinor                                             int64
	Status                                                  WithdrawalStatus
	Version                                                 int64
	CreatedAt, UpdatedAt                                    time.Time
}
type RequestWithdrawal struct {
	Referrer, Currency, PayoutMethodID string
	AmountMinor                        int64
	Method                             WithdrawalMethod
	IdempotencyKey                     string
	ExpectedVersion                    int64
}
type ReviewWithdrawal struct {
	WithdrawalID             string
	ExpectedVersion          int64
	IdempotencyKey           string
	Actor                    string
	Action                   WithdrawalStatus
	ExternalPaymentReference string
}

type Store interface {
	RecordSettledPayment(context.Context, money.PaymentSettlement) error
	RecordRefund(context.Context, money.RefundSettlement) error
	Mature(context.Context, time.Time) error
	ReadEarnings(context.Context, string, string) (Earnings, error)
	RequestWithdrawal(context.Context, RequestWithdrawal) (Withdrawal, error)
	CancelWithdrawal(context.Context, string, string, int64, string) (Withdrawal, error)
	ReviewWithdrawal(context.Context, ReviewWithdrawal) (Withdrawal, error)
}

func CommissionForCashMinor(netCashMinor int64) (int64, error) {
	if netCashMinor <= 0 {
		return 0, ErrInvalid
	}
	value := new(big.Int).Mul(big.NewInt(netCashMinor), big.NewInt(CommissionRateBPS))
	value.Quo(value, big.NewInt(BPSDenominator))
	if !value.IsInt64() || value.Sign() <= 0 {
		return 0, ErrInvalid
	}
	return value.Int64(), nil
}

// CommissionForRefundedCashMinor returns the cumulative commission that must
// be reversed for a refunded cash amount. It uses integer floor rounding and
// big.Int for the intermediate product so valid int64 minor-unit inputs cannot
// overflow during refund or maturity projection.
func CommissionForRefundedCashMinor(commissionMinor, netCashMinor, refundedMinor int64) (int64, error) {
	if commissionMinor <= 0 || netCashMinor <= 0 || refundedMinor < 0 || refundedMinor > netCashMinor {
		return 0, ErrInvalid
	}
	value := new(big.Int).Mul(big.NewInt(commissionMinor), big.NewInt(refundedMinor))
	value.Quo(value, big.NewInt(netCashMinor))
	if !value.IsInt64() || value.Sign() < 0 {
		return 0, ErrInvalid
	}
	return value.Int64(), nil
}

// CommissionRefundAdjustment returns the additional commission to reverse for
// one refund event, relative to the claim's already-refunded cash amount.
func CommissionRefundAdjustment(commissionMinor, netCashMinor, previousRefundedMinor, refundMinor int64) (int64, error) {
	if previousRefundedMinor < 0 || refundMinor <= 0 || previousRefundedMinor > netCashMinor-refundMinor {
		return 0, ErrInvalid
	}
	previous, err := CommissionForRefundedCashMinor(commissionMinor, netCashMinor, previousRefundedMinor)
	if err != nil {
		return 0, err
	}
	current, err := CommissionForRefundedCashMinor(commissionMinor, netCashMinor, previousRefundedMinor+refundMinor)
	if err != nil {
		return 0, err
	}
	return current - previous, nil
}

func ValidateWithdrawalInput(input RequestWithdrawal, spendableMinor int64) error {
	if input.Referrer == "" || input.Currency != CurrencyCNY || input.PayoutMethodID == "" || input.ExpectedVersion < 0 || input.IdempotencyKey == "" || (input.Method != MethodAlipay && input.Method != MethodBankTransfer) || input.AmountMinor < MinimumWithdrawalMinor {
		return ErrInvalid
	}
	if spendableMinor < 0 || spendableMinor < input.AmountMinor {
		return ErrInsufficient
	}
	return nil
}
