package listingkit

import (
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinSubmissionEventsWithStoreResolution(events []sheinpub.SubmissionEvent, task *Task) []sheinpub.SubmissionEvent {
	if len(events) == 0 {
		return nil
	}
	storeResolution := sheinSubmissionStoreResolutionFromTask(task)
	if storeResolution == nil {
		return append([]sheinpub.SubmissionEvent(nil), events...)
	}
	return attachSheinSubmissionEventStoreResolution(events, storeResolution)
}

func sheinSubmissionStoreResolutionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {
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

func appendSheinSubmissionEvent(pkg *sheinpub.Package, event sheinpub.SubmissionEvent) {
	submission.AppendEvent(pkg, event)
}
