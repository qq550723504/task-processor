package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type StoreResolutionSummary = sheinmarketplace.StoreResolutionSummary

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
	return sheinmarketplace.BuildStoreResolutionSummary(
		storeID,
		site,
		strategy,
		reason,
		matchedRuleKinds,
		matchedProfileID,
		manualOverride,
		fallback,
		resolvedAt,
	)
}
