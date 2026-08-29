package effectpolicy

import (
	"reflect"
	"strings"

	"task-processor/internal/imageagent"
)

func validateProviderReservation(reservation imageagent.SlotEffectV3Reservation) error {
	identity := reservation.Identity
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.OwnerUserID) == "" || strings.TrimSpace(identity.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	if identity.PlanRevision <= 0 || strings.TrimSpace(identity.SlotID) == "" || identity.Attempt <= 0 ||
		strings.TrimSpace(reservation.IdempotencyKey) == "" || strings.TrimSpace(reservation.InputFingerprint) == "" {
		return imageagent.ErrValidation
	}
	if reservation.Quote.Fingerprint == "" {
		return nil
	}
	if err := imageagent.ValidateSlotUsageQuote(reservation.Quote); err != nil {
		return err
	}
	return reservation.Policy.Allows(imageagent.UsageVector{}, imageagent.UsageVector{}, imageagent.UsageVector{})
}

func validateProviderAttemptReservation(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) error {
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(current); err != nil {
		return err
	}
	if current.Identity != reservation.Identity || current.IdempotencyKey != reservation.IdempotencyKey ||
		current.InputFingerprint != reservation.InputFingerprint || current.Policy != reservation.Policy ||
		current.Quote.Fingerprint != reservation.Quote.Fingerprint {
		return imageagent.ErrRevisionConflict
	}
	if current.Quote.Fingerprint != "" {
		if err := imageagent.ValidateSlotUsageQuote(current.Quote); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderAccountingPolicy(reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) error {
	if reservation.Quote.Fingerprint != "" && accounting.Policy != reservation.Policy {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func validateProviderReceipt(quote imageagent.SlotUsageQuote, receipt imageagent.SlotUsageReceipt) error {
	if receipt.Actual.Images < 0 || receipt.Actual.AgentSteps < 0 || receipt.Actual.ModelCalls < 0 || receipt.Actual.CostMicros < 0 ||
		receipt.Actual.Images > quote.Maximum.Images || receipt.Actual.AgentSteps > quote.Maximum.AgentSteps ||
		receipt.Actual.ModelCalls > quote.Maximum.ModelCalls || receipt.Actual.CostMicros > quote.Maximum.CostMicros {
		return imageagent.ErrRevisionConflict
	}
	if receipt.CostBasis == "" {
		return imageagent.ErrValidation
	}
	return nil
}

func sameProviderReceipt(left, right imageagent.SlotUsageReceipt) bool {
	return reflect.DeepEqual(left, right)
}
