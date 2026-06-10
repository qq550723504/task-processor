package listingkit

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	if s.submission.taskDirectSubmission != nil {
		return s.submission.taskDirectSubmission
	}
	s.submission.taskDirectSubmission = newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfig(s))
	return s.submission.taskDirectSubmission
}
