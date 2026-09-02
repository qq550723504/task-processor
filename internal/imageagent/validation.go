package imageagent

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	imagepolicy "task-processor/internal/marketplace/imagepolicy"
)

// MaxActionIDLength leaves bounded headroom for the longest projection commit
// suffix inside the PostgreSQL varchar(192) commit-key column.
const MaxActionIDLength = 128

// MaxIdempotencyKeyLength mirrors the varchar(128) persistence contract used
// by run, plan, and slot idempotency keys.
const MaxIdempotencyKeyLength = 128

// MaxJSONSafePlanRevision is the largest integer that can round-trip through
// JavaScript clients without precision loss.
const MaxJSONSafePlanRevision int64 = 1<<53 - 1

// MaxPlanSlots bounds provider fan-out and parent workflow history growth for
// newly submitted plans. Historical snapshots remain readable via ValidatePlan.
const MaxPlanSlots = 32

var actionIDPattern = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9][A-Za-z0-9._:+-]{0,%d}$`, MaxActionIDLength-1))

func ValidateImagePolicyContext(marketplace string, context ImagePolicyContext) error {
	if err := imagepolicy.ValidateProfileInput(imagepolicy.ProfileInput{
		Marketplace: marketplace, Country: context.Country, Family: context.Family, SceneCategory: context.SceneCategory,
	}); err != nil {
		return fmt.Errorf("%w: image policy key must contain canonical marketplace, country, family, and scene category", ErrValidation)
	}
	return nil
}

func ValidateActionID(value string) error {
	if len(value) > MaxActionIDLength || !actionIDPattern.MatchString(value) {
		return fmt.Errorf("action ID must be a canonical path-safe identifier of at most %d bytes", MaxActionIDLength)
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
	if plan.Revision > MaxJSONSafePlanRevision || plan.ParentRevision < 0 || plan.ParentRevision > MaxJSONSafePlanRevision {
		return fmt.Errorf("plan revision and parent revision must be JSON-safe integers")
	}
	if len(plan.Slots) > MaxPlanSlots {
		return fmt.Errorf("plan requires at most %d slots", MaxPlanSlots)
	}
	for _, slot := range plan.Slots {
		if slot.Status != SlotStatusPending {
			return fmt.Errorf("slot status must be pending")
		}
	}
	return nil
}

func ValidateInitialSubmittedPlan(plan Plan) error {
	if err := ValidateSubmittedPlan(plan); err != nil {
		return err
	}
	if plan.Revision != 1 || plan.ParentRevision != 0 {
		return fmt.Errorf("initial plan must use revision 1 with no parent revision")
	}
	return nil
}

func ValidateReplacementSubmittedPlan(expectedRevision int64, plan Plan) error {
	if expectedRevision <= 0 || expectedRevision > MaxJSONSafePlanRevision {
		return fmt.Errorf("expected revision must be a positive JSON-safe integer")
	}
	if err := ValidateSubmittedPlan(plan); err != nil {
		return err
	}
	if expectedRevision == MaxJSONSafePlanRevision || plan.ParentRevision != expectedRevision || plan.Revision != expectedRevision+1 {
		return fmt.Errorf("replacement plan must advance its parent revision by a single step")
	}
	return nil
}
