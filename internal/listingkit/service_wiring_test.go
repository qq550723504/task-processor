package listingkit

import "testing"

func TestNewServiceInitializesCollaborators(t *testing.T) {
	t.Parallel()

	svc, err := NewService(newTestServiceConfig(&stubSubmitRepo{}))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	impl, ok := svc.(*service)
	if !ok {
		t.Fatalf("service type = %T, want *service", svc)
	}
	if impl.taskLifecycle == nil {
		t.Fatal("expected taskLifecycle to be initialized")
	}
	if impl.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if impl.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if impl.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if impl.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if impl.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}
	if impl.taskTemporalSubmissionAdapter == nil {
		t.Fatal("expected taskTemporalSubmissionAdapter to be initialized")
	}
}

func TestServiceInitializeCollaboratorGroups(t *testing.T) {
	t.Parallel()

	svc := &service{repo: &stubSubmitRepo{}}

	svc.initializeTaskCollaborators()
	if svc.taskLifecycle == nil {
		t.Fatal("expected taskLifecycle to be initialized")
	}

	svc.initializeSubmitCollaborators()
	if svc.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if svc.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if svc.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if svc.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if svc.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}

	svc.initializeTemporalCollaborators()
	if svc.taskTemporalSubmissionAdapter == nil {
		t.Fatal("expected taskTemporalSubmissionAdapter to be initialized")
	}
}
