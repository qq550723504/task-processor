package imageagent

import (
	"fmt"
	"strings"
)

func IsRecoverableEffectBlockCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "recovery_requested", "recovery_start_failed",
		SlotProviderOutcomeUnknownCode, SlotStagingOutcomeUnknownCode, SlotPublicationOutcomeUnknownCode,
		SlotReviewRequiredCode, SlotReviewTransportRequiredCode,
		SlotRecoveryBlockedCode, SlotEffectPhaseInvalidCode, SlotEffectPolicyInvalidCode:
		return true
	default:
		return false
	}
}

func NormalizeRecoverableEffects(effects []RecoverableEffect) ([]RecoverableEffect, error) {
	if len(effects) == 0 {
		return nil, nil
	}
	normalized := make([]RecoverableEffect, 0, len(effects))
	seen := make(map[string]string, len(effects))
	for _, effect := range effects {
		slotID := strings.TrimSpace(effect.SlotID)
		code := strings.TrimSpace(effect.Code)
		if slotID == "" || effect.Attempt <= 0 || !IsRecoverableEffectBlockCode(code) {
			return nil, ErrValidation
		}
		key := recoverableEffectKey(slotID, effect.Attempt)
		if existingCode, exists := seen[key]; exists {
			if existingCode != code {
				return nil, fmt.Errorf("%w: conflicting recoverable effect code for %s", ErrRevisionConflict, key)
			}
			continue
		}
		seen[key] = code
		normalized = append(normalized, RecoverableEffect{SlotID: slotID, Attempt: effect.Attempt, Code: code})
	}
	return normalized, nil
}

func FindRecoverableEffect(projection RunProjection, slotID string, attempt int) (RecoverableEffect, Slot, bool) {
	slotID = strings.TrimSpace(slotID)
	if slotID == "" || attempt <= 0 {
		return RecoverableEffect{}, Slot{}, false
	}
	planSlot, hasPlanSlot := findProjectionPlanSlot(projection.Plan, slotID)
	if !hasPlanSlot {
		return RecoverableEffect{}, Slot{}, false
	}
	if normalized, err := NormalizeRecoverableEffects(projection.RecoverableEffects); err == nil {
		for _, effect := range normalized {
			if effect.SlotID != slotID || effect.Attempt != attempt {
				continue
			}
			if slotProjectionMatchesRecoverableEffect(projection.Slots, effect) {
				return effect, planSlot, true
			}
			return RecoverableEffect{}, Slot{}, false
		}
	}
	if projection.Run.Block == nil || strings.TrimSpace(projection.Run.Block.SlotID) != slotID || !IsRecoverableEffectBlockCode(projection.Run.Block.Code) {
		return RecoverableEffect{}, Slot{}, false
	}
	fallback := RecoverableEffect{SlotID: slotID, Attempt: attempt, Code: strings.TrimSpace(projection.Run.Block.Code)}
	if !slotProjectionMatchesRecoverableEffect(projection.Slots, fallback) {
		return RecoverableEffect{}, Slot{}, false
	}
	return fallback, planSlot, true
}

func recoverableEffectKey(slotID string, attempt int) string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(slotID), attempt)
}

func findProjectionPlanSlot(plan Plan, slotID string) (Slot, bool) {
	for _, slot := range plan.Slots {
		if slot.ID == slotID {
			return slot, true
		}
	}
	return Slot{}, false
}

func slotProjectionMatchesRecoverableEffect(slots []SlotProjection, effect RecoverableEffect) bool {
	for _, slot := range slots {
		if slot.Slot.ID == effect.SlotID && slot.Attempt == effect.Attempt &&
			slot.Slot.Status == SlotStatusBlocked && strings.TrimSpace(slot.ErrorCode) == effect.Code {
			return true
		}
	}
	return false
}
