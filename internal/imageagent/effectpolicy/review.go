package effectpolicy

import (
	"strings"
	"time"

	"task-processor/internal/imageagent"
)

type ReviewReservationDecision struct {
	AccountingDecision
	Acquired bool
}

func ReserveReview(current *imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, accounting AccountingSnapshot) (ReviewReservationDecision, error) {
	if err := validateReviewReservation(reservation); err != nil {
		return ReviewReservationDecision{}, err
	}
	if current == nil || current.Identity != reservation.Identity {
		return ReviewReservationDecision{}, imageagent.ErrRevisionConflict
	}
	if current.Policy != reservation.Policy {
		return ReviewReservationDecision{}, imageagent.ErrRevisionConflict
	}
	attempt := cloneSlotEffectV3Attempt(*current)
	for _, existing := range attempt.ReviewUsage {
		if existing.ActionID != reservation.ActionID {
			continue
		}
		if existing.InputFingerprint != reservation.InputFingerprint || existing.Quote.Fingerprint != reservation.Quote.Fingerprint {
			return ReviewReservationDecision{}, imageagent.ErrRevisionConflict
		}
		if existing.BudgetStatus == imageagent.SlotBudgetReleased {
			if err := accounting.Policy.Allows(accounting.Committed, accounting.Reserved, reservation.Quote.Maximum); err != nil {
				return ReviewReservationDecision{}, err
			}
			next := accounting
			var err error
			next.Reserved, err = imageagent.CheckedAddUsage(next.Reserved, reservation.Quote.Maximum)
			if err != nil {
				return ReviewReservationDecision{}, err
			}
			for index := range attempt.ReviewUsage {
				if attempt.ReviewUsage[index].ActionID == reservation.ActionID {
					attempt.ReviewUsage[index].BudgetStatus = imageagent.SlotBudgetReserved
					attempt.ReviewUsage[index].Receipt = imageagent.SlotUsageReceipt{}
				}
			}
			return ReviewReservationDecision{AccountingDecision: AccountingDecision{EffectDecision: EffectDecision{Attempt: attempt, Changed: true}, Accounting: next, AccountingChanged: true}, Acquired: true}, nil
		}
		return ReviewReservationDecision{AccountingDecision: AccountingDecision{EffectDecision: EffectDecision{Attempt: attempt}, Accounting: accounting}}, nil
	}
	if err := accounting.Policy.Allows(accounting.Committed, accounting.Reserved, reservation.Quote.Maximum); err != nil {
		return ReviewReservationDecision{}, err
	}
	next := accounting
	var err error
	next.Reserved, err = imageagent.CheckedAddUsage(next.Reserved, reservation.Quote.Maximum)
	if err != nil {
		return ReviewReservationDecision{}, err
	}
	attempt.ReviewUsage = append(attempt.ReviewUsage, imageagent.SlotReviewUsageAttempt{
		ActionID: reservation.ActionID, InputFingerprint: reservation.InputFingerprint,
		BudgetStatus: imageagent.SlotBudgetReserved, Quote: cloneSlotUsageQuote(reservation.Quote), Outcome: imageagent.SlotReviewOutcomePending,
	})
	return ReviewReservationDecision{AccountingDecision: AccountingDecision{
		EffectDecision: EffectDecision{Attempt: attempt, Changed: true}, Accounting: next, AccountingChanged: true,
	}, Acquired: true}, nil
}

func SettleReview(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, receipt imageagent.SlotUsageReceipt, accounting AccountingSnapshot, observedAt time.Time) (AccountingDecision, error) {
	decision, review, index, err := reviewAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if review.BudgetStatus == imageagent.SlotBudgetCommitted {
		if sameProviderReceipt(review.Receipt, receipt) {
			return decision, nil
		}
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if review.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if err := validateProviderReceipt(review.Quote, receipt); err != nil {
		return AccountingDecision{}, err
	}
	decision.Accounting.Reserved, err = imageagent.CheckedSubtractUsage(accounting.Reserved, review.Quote.Maximum)
	if err != nil {
		return AccountingDecision{}, err
	}
	decision.Accounting.Committed, err = imageagent.CheckedAddUsage(accounting.Committed, receipt.Actual)
	if err != nil {
		return AccountingDecision{}, err
	}
	if !accounting.StartedAt.IsZero() {
		elapsed := observedAt.Sub(accounting.StartedAt)
		if elapsed > decision.Accounting.Elapsed {
			decision.Accounting.Elapsed = elapsed
		}
	}
	decision.AccountingChanged = decision.Accounting != accounting
	decision.Attempt.ReviewUsage[index].BudgetStatus = imageagent.SlotBudgetCommitted
	decision.Attempt.ReviewUsage[index].Receipt = cloneSlotUsageReceipt(receipt)
	decision.Changed = true
	return decision, nil
}

func ReleaseReview(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	decision, review, index, err := reviewAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if review.BudgetStatus == imageagent.SlotBudgetReleased {
		return decision, nil
	}
	if review.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	decision.Accounting.Reserved, err = imageagent.CheckedSubtractUsage(accounting.Reserved, review.Quote.Maximum)
	if err != nil {
		return AccountingDecision{}, err
	}
	decision.AccountingChanged = decision.Accounting != accounting
	decision.Attempt.ReviewUsage[index].BudgetStatus = imageagent.SlotBudgetReleased
	decision.Changed = true
	return decision, nil
}

func MarkReviewUnknown(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, accounting AccountingSnapshot) (AccountingDecision, error) {
	decision, review, index, err := reviewAccountingTransition(current, reservation, accounting)
	if err != nil {
		return AccountingDecision{}, err
	}
	if review.BudgetStatus == imageagent.SlotBudgetUnknown {
		return decision, nil
	}
	if review.BudgetStatus != imageagent.SlotBudgetReserved {
		return AccountingDecision{}, imageagent.ErrRevisionConflict
	}
	if _, err := imageagent.CheckedSubtractUsage(accounting.Reserved, review.Quote.Maximum); err != nil {
		return AccountingDecision{}, err
	}
	decision.Attempt.ReviewUsage[index].BudgetStatus = imageagent.SlotBudgetUnknown
	decision.Changed = true
	return decision, nil
}

func RecordReviewOutcome(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, outcome imageagent.SlotReviewOutcome) (EffectDecision, error) {
	if err := validateReviewReservation(reservation); err != nil {
		return EffectDecision{}, err
	}
	if current.Identity != reservation.Identity || current.Policy != reservation.Policy {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if outcome != imageagent.SlotReviewOutcomeAccepted && outcome != imageagent.SlotReviewOutcomeNeedsHuman && outcome != imageagent.SlotReviewOutcomeTransportErr {
		return EffectDecision{}, imageagent.ErrValidation
	}
	cloned := cloneSlotEffectV3Attempt(current)
	for index := range cloned.ReviewUsage {
		review := &cloned.ReviewUsage[index]
		if review.ActionID != reservation.ActionID {
			continue
		}
		if review.InputFingerprint != reservation.InputFingerprint || review.Quote.Fingerprint != reservation.Quote.Fingerprint || review.BudgetStatus != imageagent.SlotBudgetCommitted {
			return EffectDecision{}, imageagent.ErrRevisionConflict
		}
		if review.Outcome == outcome {
			return EffectDecision{Attempt: cloned}, nil
		}
		if review.Outcome != "" && review.Outcome != imageagent.SlotReviewOutcomePending {
			return EffectDecision{}, imageagent.ErrRevisionConflict
		}
		review.Outcome = outcome
		return EffectDecision{Attempt: cloned, Changed: true}, nil
	}
	return EffectDecision{}, imageagent.ErrRunNotFound
}

func validateReviewReservation(reservation imageagent.SlotReviewUsageReservation) error {
	identity := reservation.Identity
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.OwnerUserID) == "" || strings.TrimSpace(identity.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	if identity.PlanRevision <= 0 || strings.TrimSpace(identity.SlotID) == "" || identity.Attempt <= 0 ||
		strings.TrimSpace(reservation.ActionID) == "" || strings.TrimSpace(reservation.InputFingerprint) == "" {
		return imageagent.ErrValidation
	}
	if err := imageagent.ValidateSlotUsageQuote(reservation.Quote); err != nil {
		return err
	}
	return reservation.Policy.Allows(imageagent.UsageVector{}, imageagent.UsageVector{}, imageagent.UsageVector{})
}

func reviewAccountingTransition(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotReviewUsageReservation, accounting AccountingSnapshot) (AccountingDecision, imageagent.SlotReviewUsageAttempt, int, error) {
	if err := validateReviewReservation(reservation); err != nil {
		return AccountingDecision{}, imageagent.SlotReviewUsageAttempt{}, -1, err
	}
	if current.Identity != reservation.Identity || current.Policy != reservation.Policy {
		return AccountingDecision{}, imageagent.SlotReviewUsageAttempt{}, -1, imageagent.ErrRevisionConflict
	}
	for index, review := range current.ReviewUsage {
		if review.ActionID == reservation.ActionID {
			if review.InputFingerprint != reservation.InputFingerprint || review.Quote.Fingerprint != reservation.Quote.Fingerprint {
				return AccountingDecision{}, imageagent.SlotReviewUsageAttempt{}, -1, imageagent.ErrRevisionConflict
			}
			return AccountingDecision{EffectDecision: EffectDecision{Attempt: cloneSlotEffectV3Attempt(current)}, Accounting: accounting}, review, index, nil
		}
	}
	return AccountingDecision{}, imageagent.SlotReviewUsageAttempt{}, -1, imageagent.ErrRunNotFound
}
