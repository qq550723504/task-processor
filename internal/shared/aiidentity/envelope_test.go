package aiidentity

import (
	"context"
	"errors"
	"testing"
)

func TestCaptureExecutionEnvelopeClassifiesRequestIdentity(t *testing.T) {
	cases := []struct {
		name     string
		identity Identity
		wantErr  error
	}{
		{name: "fully absent remains legacy anonymous", wantErr: ErrMissingIdentity},
		{name: "tenant only is partial", identity: Identity{TenantID: "tenant-a"}, wantErr: ErrIdentityIntegrity},
		{name: "user only is partial", identity: Identity{UserID: "user-a"}, wantErr: ErrIdentityIntegrity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithIdentity(context.Background(), tc.identity)
			_, err := CaptureExecutionEnvelope(ctx, "task-1", "amazon", "listing")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
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

func TestPersistedEnvelopeStateClassifiesEveryPersistedField(t *testing.T) {
	present := PersistedExecutionEnvelope{
		ExecutionIdentityVersion: CurrentEnvelopeVersion,
		ExecutionTenantID:        "tenant-a",
		ExecutionUserID:          "user-a",
		ExecutionTraceID:         "trace-a",
		ExecutionSourcePlatform:  "productenrich",
		ExecutionSourceTaskType:  "product",
	}

	cases := []struct {
		name      string
		persisted PersistedExecutionEnvelope
		want      PersistedEnvelopeState
		wantErr   bool
	}{
		{name: "absent", persisted: PersistedExecutionEnvelope{}, want: PersistedEnvelopeAbsent},
		{name: "blank fields are absent", persisted: PersistedExecutionEnvelope{ExecutionTenantID: " ", ExecutionUserID: "\t", ExecutionTraceID: "\n", ExecutionSourcePlatform: " ", ExecutionSourceTaskType: "\t"}, want: PersistedEnvelopeAbsent},
		{name: "version only", persisted: PersistedExecutionEnvelope{ExecutionIdentityVersion: CurrentEnvelopeVersion}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "tenant only", persisted: PersistedExecutionEnvelope{ExecutionTenantID: "tenant-a"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "user only", persisted: PersistedExecutionEnvelope{ExecutionUserID: "user-a"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "trace only", persisted: PersistedExecutionEnvelope{ExecutionTraceID: "trace-a"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "source platform only", persisted: PersistedExecutionEnvelope{ExecutionSourcePlatform: "productenrich"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "source task type only", persisted: PersistedExecutionEnvelope{ExecutionSourceTaskType: "product"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "unsupported version", persisted: PersistedExecutionEnvelope{ExecutionIdentityVersion: 2, ExecutionTenantID: "tenant-a", ExecutionUserID: "user-a", ExecutionSourcePlatform: "productenrich", ExecutionSourceTaskType: "product"}, want: PersistedEnvelopePartial, wantErr: true},
		{name: "present", persisted: present, want: PersistedEnvelopePresent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.persisted.State(); got != tc.want {
				t.Fatalf("State() = %v, want %v", got, tc.want)
			}
			envelope, err := tc.persisted.ExecutionEnvelope("task-a")
			if tc.wantErr {
				if !errors.Is(err, ErrIdentityIntegrity) {
					t.Fatalf("ExecutionEnvelope() error = %v, want ErrIdentityIntegrity", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExecutionEnvelope() error = %v", err)
			}
			if tc.want == PersistedEnvelopeAbsent && envelope != (ExecutionEnvelope{}) {
				t.Fatalf("ExecutionEnvelope() = %+v, want empty envelope", envelope)
			}
		})
	}
}
