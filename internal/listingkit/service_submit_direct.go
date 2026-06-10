package listingkit

import "context"

func (s *service) submitSheinTaskDirect(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	return s.taskDirectSubmissionOrDefault().submitSheinTaskDirect(ctx, taskID, task, req, opts)
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	if s.submission.taskDirectSubmission != nil {
		return s.submission.taskDirectSubmission
	}
	s.submission.taskDirectSubmission = newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfig(s))
	return s.submission.taskDirectSubmission
}
