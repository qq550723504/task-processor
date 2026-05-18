package listingkit

import (
	"context"
	"fmt"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	return &SheinSubmissionEventPage{
		TaskID: taskID,
		Items:  sheinSubmissionEventsWithStoreResolution(task.Result.Shein.SubmissionEvents, task),
	}, nil
}

func sheinSubmissionEventsWithStoreResolution(events []sheinpub.SubmissionEvent, task *Task) []sheinpub.SubmissionEvent {
	if len(events) == 0 {
		return nil
	}
	items := append([]sheinpub.SubmissionEvent(nil), events...)
	snapshot := sheinStoreResolutionSnapshotFromTask(task)
	if snapshot == nil || snapshot.StoreID <= 0 {
		return items
	}
	storeResolution := sheinSubmissionStoreResolutionFromSnapshot(snapshot)
	for idx := range items {
		if items[idx].StoreResolution != nil && items[idx].StoreResolution.StoreID > 0 {
			continue
		}
		items[idx].StoreResolution = storeResolution
	}
	return items
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

func appendSheinSubmissionEvent(pkg *sheinpub.Package, event sheinpub.SubmissionEvent) {
	if pkg == nil {
		return
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("%s-%d", event.Action, time.Now().UnixNano())
	}
	pkg.SubmissionEvents = append([]sheinpub.SubmissionEvent{event}, pkg.SubmissionEvents...)
	if len(pkg.SubmissionEvents) > 30 {
		pkg.SubmissionEvents = pkg.SubmissionEvents[:30]
	}
}
