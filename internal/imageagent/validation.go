package imageagent

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxActionIDLength leaves bounded headroom for the longest projection commit
// suffix inside the PostgreSQL varchar(192) commit-key column.
const MaxActionIDLength = 128

// MaxIdempotencyKeyLength mirrors the varchar(128) persistence contract used
// by run, plan, and slot idempotency keys.
const MaxIdempotencyKeyLength = 128

func ValidateActionID(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > MaxActionIDLength {
		return fmt.Errorf("action ID must be canonical and at most %d bytes", MaxActionIDLength)
	}
	return nil
}

func validatePersistedIdempotencyKey(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s idempotency key must not be empty", kind)
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxIdempotencyKeyLength {
		return fmt.Errorf("%s idempotency key must be valid UTF-8 and at most %d characters", kind, MaxIdempotencyKeyLength)
	}
	return nil
}

var knownSlotRoles = map[SlotRole]struct{}{
	SlotRoleMain: {}, SlotRoleScene: {}, SlotRoleDetail: {}, SlotRoleSellingPoint: {}, SlotRoleSize: {},
}

func ValidatePlan(plan Plan) error {
	if plan.Revision <= 0 {
		return fmt.Errorf("revision must be positive")
	}
	if err := validatePersistedIdempotencyKey("plan", plan.IdempotencyKey); err != nil {
		return err
	}

	sources := make(map[string]struct{}, len(plan.SourceAssetIDs))
	styles := make(map[string]struct{}, len(plan.StyleReferenceIDs))
	for _, rawID := range plan.SourceAssetIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		sources[id] = struct{}{}
	}
	if len(sources) == 0 {
		return fmt.Errorf("plan requires at least one source asset")
	}
	for _, rawID := range plan.StyleReferenceIDs {
		id := strings.TrimSpace(rawID)
		if id != "" {
			styles[id] = struct{}{}
		}
	}
	if len(plan.Slots) == 0 {
		return fmt.Errorf("plan requires at least one slot")
	}

	slotIDs := make(map[string]struct{}, len(plan.Slots))
	idempotencyKeys := make(map[string]struct{}, len(plan.Slots))
	mainSlots := 0
	for _, slot := range plan.Slots {
		rawID := slot.ID
		id := strings.TrimSpace(rawID)
		if id == "" {
			return fmt.Errorf("slot id must not be empty")
		}
		if rawID != id {
			return fmt.Errorf("%w: slot id %q must be canonical", ErrValidation, rawID)
		}
		if err := ValidateArtifactKeyIdentifier(rawID); err != nil {
			return fmt.Errorf("%w: slot id %q cannot be used in a durable artifact key", err, id)
		}
		if _, exists := slotIDs[id]; exists {
			return fmt.Errorf("duplicate slot id %q", id)
		}
		slotIDs[id] = struct{}{}
		if _, known := knownSlotRoles[slot.Role]; !known {
			return fmt.Errorf("unknown slot role %q", slot.Role)
		}
		if slot.Role == SlotRoleMain {
			mainSlots++
		}

		key := strings.TrimSpace(slot.IdempotencyKey)
		if err := validatePersistedIdempotencyKey("slot", slot.IdempotencyKey); err != nil {
			return err
		}
		if _, exists := idempotencyKeys[key]; exists {
			return fmt.Errorf("duplicate idempotency key %q", key)
		}
		idempotencyKeys[key] = struct{}{}

		slotSources := 0
		for _, rawSourceID := range slot.SourceAssetIDs {
			sourceID := strings.TrimSpace(rawSourceID)
			if sourceID == "" {
				continue
			}
			if _, contained := sources[sourceID]; !contained {
				return fmt.Errorf("source asset reference %q is not in plan", sourceID)
			}
			slotSources++
		}
		if slotSources == 0 {
			return fmt.Errorf("slot requires at least one source asset")
		}
		for _, rawStyleID := range slot.StyleReferenceIDs {
			styleID := strings.TrimSpace(rawStyleID)
			if styleID == "" {
				continue
			}
			if _, contained := styles[styleID]; !contained {
				return fmt.Errorf("style reference %q is not in plan", styleID)
			}
		}
		if slot.Status != "" && slot.Status != SlotStatusPending {
			return fmt.Errorf("slot status must be pending")
		}
	}
	if mainSlots != 1 {
		return fmt.Errorf("plan requires exactly one main slot")
	}
	return nil
}

// ValidateSubmittedPlan applies the stricter command-ingress contract while
// ValidatePlan remains able to read pre-contract workflow histories whose
// pending status was omitted from the serialized plan.
func ValidateSubmittedPlan(plan Plan) error {
	if err := ValidatePlan(plan); err != nil {
		return err
	}
	for _, slot := range plan.Slots {
		if slot.Status != SlotStatusPending {
			return fmt.Errorf("slot status must be pending")
		}
	}
	return nil
}
