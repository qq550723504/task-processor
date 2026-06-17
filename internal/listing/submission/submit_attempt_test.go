package submission

import (
	"testing"
	"time"
)

func TestNormalizeSubmitAttemptMapsPlanFields(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)

	got := NormalizeSubmitAttempt(SubmitAttempt{
		AttemptID:      " attempt-1 ",
		TaskID:         " task-1 ",
		TenantID:       " tenant-1 ",
		TargetPlatform: " SHEIN ",
		Action:         " SAVE_DRAFT ",
		Status:         " RUNNING ",
		Phase:          " UPLOAD_IMAGES ",
		IdempotencyKey: " idem-1 ",
		RemoteRecord: SubmitRemoteRecord{
			ProductID: " product-1 ",
			DraftID:   " draft-1 ",
			PublishID: " publish-1 ",
		},
		Error: &SubmitErrorDetail{
			Code:        " remote_timeout ",
			Message:     " timed out ",
			Recoverable: true,
		},
		CreatedAt:  startedAt,
		UpdatedAt:  startedAt.Add(time.Minute),
		FinishedAt: &finishedAt,
	})

	if got.AttemptID != "attempt-1" || got.TaskID != "task-1" || got.TenantID != "tenant-1" {
		t.Fatalf("identity fields = %+v, want trimmed values", got)
	}
	if got.TargetPlatform != "shein" || got.Action != SubmitActionSaveDraft {
		t.Fatalf("target = %s/%s, want shein/save_draft", got.TargetPlatform, got.Action)
	}
	if got.Status != SubmitStatusRunning || got.Phase != SubmitPhaseUploadImages {
		t.Fatalf("state = %s/%s, want running/upload_images", got.Status, got.Phase)
	}
	if got.IdempotencyKey != "idem-1" {
		t.Fatalf("idempotency key = %q, want idem-1", got.IdempotencyKey)
	}
	if got.RemoteRecord.ProductID != "product-1" || got.RemoteRecord.DraftID != "draft-1" || got.RemoteRecord.PublishID != "publish-1" {
		t.Fatalf("remote record = %+v, want trimmed ids", got.RemoteRecord)
	}
	if got.Error == nil || got.Error.Code != "remote_timeout" || got.Error.Message != "timed out" || !got.Error.Recoverable {
		t.Fatalf("error = %+v, want normalized recoverable error", got.Error)
	}
}

func TestValidateSubmitAttemptRequiresPlanIdentityAndState(t *testing.T) {
	t.Parallel()

	valid := SubmitAttempt{
		AttemptID:      "attempt-1",
		TaskID:         "task-1",
		TargetPlatform: "shein",
		Action:         SubmitActionPublish,
		Status:         SubmitStatusPending,
		Phase:          SubmitPhaseValidate,
		IdempotencyKey: "idem-1",
		CreatedAt:      time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC),
	}

	if err := ValidateSubmitAttempt(valid); err != nil {
		t.Fatalf("ValidateSubmitAttempt(valid) = %v, want nil", err)
	}

	cases := map[string]SubmitAttempt{
		"task_id":         withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.TaskID = "" }),
		"target_platform": withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.TargetPlatform = "" }),
		"action":          withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.Action = "delete" }),
		"status":          withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.Status = "done" }),
		"phase":           withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.Phase = "ship" }),
		"idempotency_key": withSubmitAttemptField(valid, func(a *SubmitAttempt) { a.IdempotencyKey = "" }),
	}

	for wantField, attempt := range cases {
		wantField, attempt := wantField, attempt
		t.Run(wantField, func(t *testing.T) {
			t.Parallel()

			err := ValidateSubmitAttempt(attempt)
			if err == nil {
				t.Fatalf("ValidateSubmitAttempt() = nil, want field error %q", wantField)
			}
			var fieldErr *SubmitAttemptValidationError
			if !AsSubmitAttemptValidationError(err, &fieldErr) || fieldErr.Field != wantField {
				t.Fatalf("ValidateSubmitAttempt() = %v, want field error %q", err, wantField)
			}
		})
	}
}

func withSubmitAttemptField(attempt SubmitAttempt, mutate func(*SubmitAttempt)) SubmitAttempt {
	mutate(&attempt)
	return attempt
}
