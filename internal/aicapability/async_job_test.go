package aicapability

import (
	"errors"
	"testing"
	"time"
)

func TestAsyncJobBindingValidationRejectsEmptyJobID(t *testing.T) {
	err := ValidateAsyncJobBinding(AsyncJobBinding{ProviderID: "provider-a"})
	if !errors.Is(err, ErrAsyncJobBindingInvalid) {
		t.Fatalf("ValidateAsyncJobBinding() error = %v, want ErrAsyncJobBindingInvalid", err)
	}
}

func TestAsyncJobBindingValidationAcceptsRoutingMetadata(t *testing.T) {
	err := ValidateAsyncJobBinding(AsyncJobBinding{
		JobID: "job-a", TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage,
		Operation: OperationAsyncImageGenerate, ProviderID: "provider-a", ModelID: "model-a",
		RoutingKey: "route-a", SubmittedAt: time.Now().UTC(), Status: "queued",
	})
	if err != nil {
		t.Fatalf("ValidateAsyncJobBinding() error = %v", err)
	}
}

func TestAsyncJobBindingSentinelsAreDistinct(t *testing.T) {
	if ErrAsyncJobBindingInvalid == nil || ErrAsyncJobBindingNotFound == nil || ErrAsyncJobBindingConflict == nil {
		t.Fatal("async job binding sentinel errors must be non-nil")
	}
	if ErrAsyncJobBindingInvalid == ErrAsyncJobBindingNotFound || ErrAsyncJobBindingInvalid == ErrAsyncJobBindingConflict || ErrAsyncJobBindingNotFound == ErrAsyncJobBindingConflict {
		t.Fatal("async job binding sentinel errors must be distinct")
	}
}
