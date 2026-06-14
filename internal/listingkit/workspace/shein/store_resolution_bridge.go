package shein

import (
	"time"

	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

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
	return sheinmarketplace.BuildSubmissionStoreResolution(
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

func AttachSubmissionEventStoreResolution(
	events []sheinpub.SubmissionEvent,
	storeResolution *sheinpub.SubmissionStoreResolution,
) []sheinpub.SubmissionEvent {
	return sheinmarketplace.AttachSubmissionEventStoreResolution(events, storeResolution)
}
