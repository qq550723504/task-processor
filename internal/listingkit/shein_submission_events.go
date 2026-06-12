package listingkit

import (
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
	return sheinSubmissionStoreResolutionFromSnapshotValue(snapshot)
}

func appendSheinSubmissionEvent(pkg *sheinpub.Package, event sheinpub.SubmissionEvent) {
	submission.AppendEvent(pkg, event)
}
