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
	if strings.TrimSpace(plan.IdempotencyKey) == "" {
		return fmt.Errorf("plan idempotency key must not be empty")
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
		if key == "" {
			return fmt.Errorf("slot idempotency key must not be empty")
		}
		if _, exists := idempotencyKeys[key]; exists {
			return fmt.Errorf("duplicate idempotency key %q", key)
		}
		idempotencyKeys[key] = struct{}{}

		for _, rawSourceID := range slot.SourceAssetIDs {
			sourceID := strings.TrimSpace(rawSourceID)
			if sourceID == "" {
				continue
			}
			if _, contained := sources[sourceID]; !contained {
				return fmt.Errorf("source asset reference %q is not in plan", sourceID)
			}
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
	}
	return nil
}
