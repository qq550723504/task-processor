package effectpolicy

import (
	"time"

	"task-processor/internal/imageagent"
)

func ReserveProvider(current *imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (ProviderReservationDecision, error) {
	if err := validateProviderReservation(reservation); err != nil {
		return ProviderReservationDecision{}, err
	}
	if current == nil {
		attempt := imageagent.SlotEffectV3Attempt{
			Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
			Phase: imageagent.SlotEffectV3ProviderClaimed, Policy: reservation.Policy, Quote: cloneSlotUsageQuote(reservation.Quote),
		}
		decision := ProviderReservationDecision{AccountingDecision: AccountingDecision{EffectDecision: EffectDecision{Attempt: attempt, Changed: true}, Accounting: accounting}, Acquired: true}
		if reservation.Quote.Fingerprint != "" {
			next, err := reserveProviderAccounting(reservation, accounting)
			if err != nil {
				return ProviderReservationDecision{}, err
			}
			decision.Attempt.BudgetStatus = imageagent.SlotBudgetReserved
			decision.Accounting = next
			decision.AccountingChanged = next != accounting
		}
		return decision, nil
	}

	attempt := cloneSlotEffectV3Attempt(*current)
	if err := validateProviderAttemptReservation(attempt, reservation); err != nil {
		return ProviderReservationDecision{}, err
	}
	decision := ProviderReservationDecision{AccountingDecision: AccountingDecision{EffectDecision: EffectDecision{Attempt: attempt}, Accounting: accounting}}
	if attempt.Phase == imageagent.SlotEffectV3ProviderNotDispatched {
		if reservation.Quote.Fingerprint != "" {
			next, err := reserveProviderAccounting(reservation, accounting)
			if err != nil {
				return ProviderReservationDecision{}, err
			}
			decision.Accounting = next
			decision.AccountingChanged = next != accounting
			decision.Attempt.BudgetStatus = imageagent.SlotBudgetReserved
		}
		decision.Attempt.Phase = imageagent.SlotEffectV3ProviderClaimed
		decision.Changed = true
		decision.Acquired = true
		return decision, nil
	}
	if attempt.BudgetStatus == imageagent.SlotBudgetReleased && reservation.Quote.Fingerprint != "" {
		next, err := reserveProviderAccounting(reservation, accounting)
		if err != nil {
			return ProviderReservationDecision{}, err
		}
		decision.Accounting = next
		decision.AccountingChanged = next != accounting
		decision.Attempt.BudgetStatus = imageagent.SlotBudgetReserved
		decision.Changed = true
		decision.Acquired = true
	}
	return decision, nil
}

func RecordProviderNotDispatched(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	decision, err := providerAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if decision.Attempt.Phase == imageagent.SlotEffectV3ProviderNotDispatched {
		return decision, nil
	}
	if decision.Attempt.Phase != imageagent.SlotEffectV3ProviderClaimed {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if reservation.Quote.Fingerprint != "" {
		if decision.Attempt.BudgetStatus != imageagent.SlotBudgetReserved {
			return AccountingDecision{}, imageagent.ErrRevisionConflict
		}
		nextReserved, subtractErr := imageagent.CheckedSubtractUsage(accounting.Reserved, decision.Attempt.Quote.Maximum)
		if subtractErr != nil {
			return AccountingDecision{}, subtractErr
		}
		decision.Accounting.Reserved = nextReserved
		decision.AccountingChanged = decision.Accounting != accounting
		decision.Attempt.BudgetStatus = imageagent.SlotBudgetReleased
	} else if decision.Attempt.BudgetStatus != "" {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	decision.Attempt.Phase = imageagent.SlotEffectV3ProviderNotDispatched
	decision.Changed = true
	return decision, nil
}

func SettleProvider(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt, accounting AccountingSnapshot, observedAt time.Time) (AccountingDecision, error) {
	decision, err := providerAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if decision.Attempt.BudgetStatus == imageagent.SlotBudgetCommitted {
		if sameProviderReceipt(decision.Attempt.Receipt, receipt) {
			return decision, nil
		}
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if decision.Attempt.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if err := validateProviderReceipt(decision.Attempt.Quote, receipt); err != nil {
		return AccountingDecision{}, err
	}
	nextReserved, err := imageagent.CheckedSubtractUsage(accounting.Reserved, decision.Attempt.Quote.Maximum)
	if err != nil {
		return AccountingDecision{}, err
	}
	nextCommitted, err := imageagent.CheckedAddUsage(accounting.Committed, receipt.Actual)
	if err != nil {
		return AccountingDecision{}, err
	}
	decision.Accounting.Reserved = nextReserved
	decision.Accounting.Committed = nextCommitted
	if !accounting.StartedAt.IsZero() {
		elapsed := observedAt.Sub(accounting.StartedAt)
		if elapsed > decision.Accounting.Elapsed {
			decision.Accounting.Elapsed = elapsed
		}
	}
	decision.AccountingChanged = decision.Accounting != accounting
	decision.Attempt.BudgetStatus = imageagent.SlotBudgetCommitted
	decision.Attempt.Receipt = cloneSlotUsageReceipt(receipt)
	decision.Changed = true
	return decision, nil
}

func ReleaseProviderBudget(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	decision, err := providerAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if decision.Attempt.BudgetStatus == imageagent.SlotBudgetReleased {
		return decision, nil
	}
	if decision.Attempt.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	nextReserved, err := imageagent.CheckedSubtractUsage(accounting.Reserved, decision.Attempt.Quote.Maximum)
	if err != nil {
		return AccountingDecision{}, err
	}
	decision.Accounting.Reserved = nextReserved
	decision.AccountingChanged = decision.Accounting != accounting
	decision.Attempt.BudgetStatus = imageagent.SlotBudgetReleased
	decision.Changed = true
	return decision, nil
}

func MarkProviderBudgetUnknown(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	decision, err := providerAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if decision.Attempt.BudgetStatus == imageagent.SlotBudgetUnknown {
		return decision, nil
	}
	if decision.Attempt.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if _, err := imageagent.CheckedSubtractUsage(accounting.Reserved, decision.Attempt.Quote.Maximum); err != nil {
		return AccountingDecision{}, err
	}
	decision.Attempt.BudgetStatus = imageagent.SlotBudgetUnknown
	decision.Changed = true
	return decision, nil
}

func providerAccountingTransition(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	if err := validateProviderReservation(reservation); err != nil {
		return AccountingDecision{}, err
	}
	if err := validateProviderAttemptReservation(current, reservation); err != nil {
		return AccountingDecision{}, err
	}
	return AccountingDecision{EffectDecision: EffectDecision{Attempt: cloneSlotEffectV3Attempt(current)}, Accounting: accounting}, nil
}

func reserveProviderAccounting(reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingSnapshot, error) {
	if err := validateProviderAccountingPolicy(reservation, accounting); err != nil {
		return AccountingSnapshot{}, err
	}
	if err := accounting.Policy.Allows(accounting.Committed, accounting.Reserved, reservation.Quote.Maximum); err != nil {
		return AccountingSnapshot{}, err
	}
	reserved, err := imageagent.CheckedAddUsage(accounting.Reserved, reservation.Quote.Maximum)
	if err != nil {
		return AccountingSnapshot{}, err
	}
	accounting.Reserved = reserved
	return accounting, nil
}

func cloneSlotUsageQuote(quote imageagent.SlotUsageQuote) imageagent.SlotUsageQuote {
	quote.Operations = cloneSlice(quote.Operations)
	return quote
}

func cloneSlotUsageReceipt(receipt imageagent.SlotUsageReceipt) imageagent.SlotUsageReceipt {
	receipt.ProviderRequestIDs = cloneSlice(receipt.ProviderRequestIDs)
	return receipt
}
