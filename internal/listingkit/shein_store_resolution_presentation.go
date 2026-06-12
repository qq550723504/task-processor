package listingkit

import (
	"time"

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
	return &SheinStoreResolutionSummary{
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

func sheinSubmissionStoreResolutionFromSnapshotValue(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {
	if snapshot == nil || snapshot.StoreID <= 0 {
		return nil
	}
	var resolvedAt *time.Time
	if !snapshot.ResolvedAt.IsZero() {
		value := snapshot.ResolvedAt
		resolvedAt = &value
	}
	return &sheinpub.SubmissionStoreResolution{
		StoreID:          snapshot.StoreID,
		Site:             snapshot.Site,
		Strategy:         snapshot.Strategy,
		Reason:           snapshot.Reason,
		MatchedRuleKinds: append([]string(nil), snapshot.MatchedRuleKinds...),
		MatchedProfileID: snapshot.MatchedProfileID,
		ManualOverride:   snapshot.ManualOverride,
		Fallback:         snapshot.Fallback,
		ResolvedAt:       resolvedAt,
	}
}

func sheinSubmissionStoreResolutionFromTask(task *Task) *sheinpub.SubmissionStoreResolution {
	return sheinSubmissionStoreResolutionFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
}
