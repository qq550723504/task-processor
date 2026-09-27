package referral

import (
	"errors"
	"testing"
	"time"

	economics "task-processor/internal/referraleconomics"
)

func TestCommittedWithdrawalReplayRequiresExactSubjectAndFingerprint(t *testing.T) {
	now := time.Date(2026, 9, 27, 2, 0, 0, 0, time.UTC)
	request := economics.WithdrawalReplayRequest{
		Referrer: "person-1", Currency: economics.CurrencyCNY, PayoutMethodID: "method-1",
		AmountMinor: economics.MinimumWithdrawalMinor, IdempotencyKey: "request-key", ExpectedVersion: 7,
	}
	row := withdrawalRow{
		ID: "withdrawal-1", Referrer: request.Referrer, PayoutMethodID: request.PayoutMethodID,
		Currency: request.Currency, Method: string(economics.MethodAlipay), AmountMinor: request.AmountMinor,
		Status: string(economics.WithdrawalRequested), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	full := economics.RequestWithdrawal{
		Referrer: request.Referrer, Currency: request.Currency, PayoutMethodID: request.PayoutMethodID,
		AmountMinor: request.AmountMinor, Method: economics.MethodAlipay,
		IdempotencyKey: request.IdempotencyKey, ExpectedVersion: request.ExpectedVersion,
	}
	op := withdrawalOperationRow{IdempotencyKey: request.IdempotencyKey, WithdrawalID: row.ID, Fingerprint: withdrawalFingerprint(full)}

	got, err := committedWithdrawalReplay(request, op, row)
	if err != nil || got.ID != row.ID || got.Referrer != request.Referrer {
		t.Fatalf("exact replay=%+v err=%v", got, err)
	}

	foreign := request
	foreign.Referrer = "person-2"
	if _, err = committedWithdrawalReplay(foreign, op, row); !errors.Is(err, economics.ErrIdempotencyConflict) {
		t.Fatalf("cross-subject replay=%v", err)
	}

	changed := request
	changed.AmountMinor++
	if _, err = committedWithdrawalReplay(changed, op, row); !errors.Is(err, economics.ErrIdempotencyConflict) {
		t.Fatalf("changed replay=%v", err)
	}

	wrongRow := row
	wrongRow.ID = "withdrawal-other"
	if _, err = committedWithdrawalReplay(request, op, wrongRow); !errors.Is(err, economics.ErrUnavailable) {
		t.Fatalf("operation/withdrawal mismatch=%v", err)
	}
}
