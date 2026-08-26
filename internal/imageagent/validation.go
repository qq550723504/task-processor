package imageagent

import (
	"fmt"
	"strings"
)

var knownSlotRoles = map[SlotRole]struct{}{
	SlotRoleMain: {}, SlotRoleScene: {}, SlotRoleDetail: {}, SlotRoleSellingPoint: {}, SlotRoleSize: {},
}

func ValidatePlan(plan Plan) error {
	if plan.Revision <= 0 {
		return fmt.Errorf("revision must be positive")
	}

	sources := make(map[string]struct{}, len(plan.SourceAssetIDs))
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
	if len(plan.Slots) == 0 {
		return fmt.Errorf("plan requires at least one slot")
	}

	slotIDs := make(map[string]struct{}, len(plan.Slots))
	idempotencyKeys := make(map[string]struct{}, len(plan.Slots))
	for _, slot := range plan.Slots {
		id := strings.TrimSpace(slot.ID)
		if id == "" {
			return fmt.Errorf("slot id must not be empty")
		}
		if _, exists := slotIDs[id]; exists {
			return fmt.Errorf("duplicate slot id %q", id)
		}
		slotIDs[id] = struct{}{}
		if _, known := knownSlotRoles[slot.Role]; !known {
			return fmt.Errorf("unknown slot role %q", slot.Role)
		}

		key := strings.TrimSpace(slot.IdempotencyKey)
		if key != "" {
			if _, exists := idempotencyKeys[key]; exists {
				return fmt.Errorf("duplicate idempotency key %q", key)
			}
			idempotencyKeys[key] = struct{}{}
		}

		for _, rawSourceID := range slot.SourceAssetIDs {
			sourceID := strings.TrimSpace(rawSourceID)
			if sourceID == "" {
				continue
			}
			if _, contained := sources[sourceID]; !contained {
				return fmt.Errorf("source asset reference %q is not in plan", sourceID)
			}
		}
	}
	return nil
}
