package listingkit

import (
	"time"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinStoreResolutionSummaryFromSnapshotValue(snapshot *SheinStoreResolutionSnapshot) *SheinStoreResolutionSummary {
	if snapshot == nil || snapshot.StoreID <= 0 {
		return nil
	}
	var resolvedAt string
	if !snapshot.ResolvedAt.IsZero() {
		resolvedAt = snapshot.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return buildSheinStoreResolutionSummaryValue(
		snapshot.StoreID,
		snapshot.Site,
		snapshot.Strategy,
		snapshot.Reason,
		snapshot.MatchedRuleKinds,
		snapshot.MatchedProfileID,
		snapshot.ManualOverride,
		snapshot.Fallback,
		resolvedAt,
	)
}

func sheinStoreResolutionSummaryFromTask(task *Task) *SheinStoreResolutionSummary {
	return sheinStoreResolutionSummaryFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
}

func buildSheinStoreResolutionSummaryValue(
	storeID int64,
	site string,
	strategy string,
	reason string,
	matchedRuleKinds []string,
	matchedProfileID int64,
	manualOverride bool,
	fallback bool,
	resolvedAt string,
) *SheinStoreResolutionSummary {
	return sheinworkspace.BuildStoreResolutionSummary(
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

func sheinSubmissionStoreResolutionFromSnapshotValue(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {
	if snapshot == nil {
		return nil
	}
	var resolvedAt *time.Time
	if !snapshot.ResolvedAt.IsZero() {
		value := snapshot.ResolvedAt
		resolvedAt = &value
	}
	return sheinworkspace.BuildSubmissionStoreResolution(
		snapshot.StoreID,
		snapshot.Site,
		snapshot.Strategy,
		snapshot.Reason,
		snapshot.MatchedRuleKinds,
		snapshot.MatchedProfileID,
		snapshot.ManualOverride,
		snapshot.Fallback,
		resolvedAt,
	)
}

func sheinSubmissionStoreResolutionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {
	return sheinSubmissionStoreResolutionFromSnapshotValue(snapshot)
}

func sheinSubmissionStoreResolutionFromTask(task *Task) *sheinpub.SubmissionStoreResolution {
	return sheinSubmissionStoreResolutionFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
}
