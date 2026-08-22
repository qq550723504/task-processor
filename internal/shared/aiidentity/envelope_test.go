package aiidentity

import (
	"context"
	"errors"
	"testing"
)

func TestCaptureExecutionEnvelopeRequiresVerifiedIdentity(t *testing.T) {
	_, err := CaptureExecutionEnvelope(context.Background(), "task-1", "amazon", "listing")
	if !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("error = %v, want ErrMissingIdentity", err)
	}
}

func TestCaptureAndRestoreExecutionEnvelope(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{
		TenantID: " tenant-a ",
		UserID:   " user-a ",
		TraceID:  " trace-a ",
	})

	envelope, err := CaptureExecutionEnvelope(ctx, "task-a", "amazon", "listing")
	if err != nil {
		t.Fatalf("CaptureExecutionEnvelope: %v", err)
	}
	if envelope.Version != CurrentEnvelopeVersion || envelope.BusinessTaskID != "task-a" {
		t.Fatalf("envelope = %+v", envelope)
	}

	restored, err := RestoreExecutionEnvelope(context.Background(), envelope, "task-a")
	if err != nil {
		t.Fatalf("RestoreExecutionEnvelope: %v", err)
	}
	identity := FromContext(restored)
	if identity.TenantID != "tenant-a" || identity.UserID != "user-a" || identity.BusinessTaskID != "task-a" || identity.TraceID != "trace-a" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestRestoreExecutionEnvelopeRejectsTaskMismatch(t *testing.T) {
	envelope := ExecutionEnvelope{
		Version:        CurrentEnvelopeVersion,
		TenantID:       "tenant-a",
		UserID:         "user-a",
		BusinessTaskID: "task-a",
		SourcePlatform: "amazon",
		SourceTaskType: "listing",
	}
	_, err := RestoreExecutionEnvelope(context.Background(), envelope, "task-b")
	if !errors.Is(err, ErrIdentityIntegrity) {
		t.Fatalf("error = %v, want ErrIdentityIntegrity", err)
	}
}

func TestExecutionEnvelopeValidationRejectsPartialAndUnsupportedValues(t *testing.T) {
	cases := []ExecutionEnvelope{
		{Version: 2, TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-a", SourcePlatform: "amazon", SourceTaskType: "listing"},
		{Version: CurrentEnvelopeVersion, UserID: "user-a", BusinessTaskID: "task-a", SourcePlatform: "amazon", SourceTaskType: "listing"},
		{Version: CurrentEnvelopeVersion, TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-a", SourcePlatform: "unknown", SourceTaskType: "listing"},
	}
	for _, envelope := range cases {
		if err := envelope.Validate(); !errors.Is(err, ErrIdentityIntegrity) {
			t.Errorf("Validate(%+v) = %v, want ErrIdentityIntegrity", envelope, err)
		}
	}
}

func TestPersistedExecutionEnvelopeRoundTrip(t *testing.T) {
	want := ExecutionEnvelope{
		Version:        CurrentEnvelopeVersion,
		TenantID:       "tenant-a",
		UserID:         "user-a",
		BusinessTaskID: "task-a",
		TraceID:        "trace-a",
		SourcePlatform: "productimage",
		SourceTaskType: "image",
	}
	persisted := PersistedExecutionEnvelopeFrom(want)
	got, err := persisted.ExecutionEnvelope("task-a")
	if err != nil {
		t.Fatalf("ExecutionEnvelope: %v", err)
	}
	if got != want {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
}
