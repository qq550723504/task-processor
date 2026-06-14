package workspace

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
