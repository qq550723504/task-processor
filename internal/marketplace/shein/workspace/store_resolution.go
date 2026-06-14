package workspace

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type StoreResolutionSummary struct {
	StoreID          int64    `json:"store_id,omitempty"`
	Site             string   `json:"site,omitempty"`
	Strategy         string   `json:"strategy,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	MatchedRuleKinds []string `json:"matched_rule_kinds,omitempty"`
	MatchedProfileID int64    `json:"matched_profile_id,omitempty"`
	ManualOverride   bool     `json:"manual_override,omitempty"`
	Fallback         bool     `json:"fallback,omitempty"`
	ResolvedAt       string   `json:"resolved_at,omitempty"`
}

func BuildStoreResolutionSummary(
	storeID int64,
	site string,
	strategy string,
	reason string,
	matchedRuleKinds []string,
	matchedProfileID int64,
	manualOverride bool,
	fallback bool,
	resolvedAt string,
) *StoreResolutionSummary {
	return &StoreResolutionSummary{
		StoreID:          storeID,
		Site:             site,
		Strategy:         strategy,
		Reason:           reason,
		MatchedRuleKinds: append([]string(nil), matchedRuleKinds...),
		MatchedProfileID: matchedProfileID,
		ManualOverride:   manualOverride,
		Fallback:         fallback,
		ResolvedAt:       resolvedAt,
	}
}

func BuildSubmissionStoreResolution(
	storeID int64,
	site string,
	strategy string,
	reason string,
	matchedRuleKinds []string,
	matchedProfileID int64,
	manualOverride bool,
	fallback bool,
	resolvedAt *time.Time,
) *sheinpub.SubmissionStoreResolution {
	if storeID <= 0 {
		return nil
	}
	var resolvedAtValue *time.Time
	if resolvedAt != nil && !resolvedAt.IsZero() {
		value := *resolvedAt
		resolvedAtValue = &value
	}
	return &sheinpub.SubmissionStoreResolution{
		StoreID:          storeID,
		Site:             site,
		Strategy:         strategy,
		Reason:           reason,
		MatchedRuleKinds: append([]string(nil), matchedRuleKinds...),
		MatchedProfileID: matchedProfileID,
		ManualOverride:   manualOverride,
		Fallback:         fallback,
		ResolvedAt:       resolvedAtValue,
	}
}

func AttachSubmissionEventStoreResolution(
	events []sheinpub.SubmissionEvent,
	storeResolution *sheinpub.SubmissionStoreResolution,
) []sheinpub.SubmissionEvent {
	if len(events) == 0 {
		return nil
	}
	items := append([]sheinpub.SubmissionEvent(nil), events...)
	if storeResolution == nil {
		return items
	}
	for idx := range items {
		if items[idx].StoreResolution != nil && items[idx].StoreResolution.StoreID > 0 {
			continue
		}
		items[idx].StoreResolution = storeResolution
	}
	return items
}
