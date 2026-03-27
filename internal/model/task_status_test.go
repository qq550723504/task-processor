package model

import "testing"

func TestParseTaskStatus(t *testing.T) {
	status, err := ParseTaskStatus(1)
	if err != nil {
		t.Fatalf("ParseTaskStatus returned error: %v", err)
	}
	if status != TaskStatusProcessing {
		t.Fatalf("ParseTaskStatus = %v, want %v", status, TaskStatusProcessing)
	}
}

func TestParseTaskStatusUnknown(t *testing.T) {
	if _, err := ParseTaskStatus(99); err == nil {
		t.Fatal("ParseTaskStatus should reject unknown status code")
	}
}

func TestValidateTaskStatusTransition(t *testing.T) {
	cases := []struct {
		name    string
		from    TaskStatus
		to      TaskStatus
		wantErr bool
	}{
		{name: "pending to processing", from: TaskStatusPending, to: TaskStatusProcessing},
		{name: "processing to published", from: TaskStatusProcessing, to: TaskStatusPublished},
		{name: "processing to paused", from: TaskStatusProcessing, to: TaskStatusPaused},
		{name: "processing to pending retry", from: TaskStatusProcessing, to: TaskStatusPendingRetry},
		{name: "published to pending retry is invalid", from: TaskStatusPublished, to: TaskStatusPendingRetry, wantErr: true},
		{name: "terminal to processing is invalid", from: TaskStatusDraft, to: TaskStatusProcessing, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskStatusTransition(tc.from, tc.to)
			if tc.wantErr && err == nil {
				t.Fatal("ValidateTaskStatusTransition should fail")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateTaskStatusTransition returned error: %v", err)
			}
		})
	}
}

func TestValidateTaskStatusTransitionCode(t *testing.T) {
	if err := ValidateTaskStatusTransitionCode(int16(TaskStatusPendingRetry), TaskStatusProcessing); err != nil {
		t.Fatalf("ValidateTaskStatusTransitionCode returned error: %v", err)
	}

	if err := ValidateTaskStatusTransitionCode(int16(TaskStatusPublished), TaskStatusProcessing); err == nil {
		t.Fatal("ValidateTaskStatusTransitionCode should reject invalid transition")
	}
}
