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
		Items:  append([]sheinpub.SubmissionEvent(nil), task.Result.Shein.SubmissionEvents...),
	}, nil
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
