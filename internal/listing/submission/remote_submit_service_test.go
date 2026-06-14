package submission

import (
	"context"
	"errors"
	"testing"
)

func TestRemoteSubmitServiceSubmit(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("remote failed")
	service := NewRemoteSubmitService(RemoteSubmitServiceConfig[string, string, string, string, string]{
		PrepareState: func(pkg, action, requestID, product, snapshot string) (string, string) {
			if pkg != "pkg" || action != "publish" || requestID != "req-1" || product != "product" || snapshot != "snapshot-in" {
				t.Fatalf("unexpected prepare args")
			}
			return "SUP-1", "snapshot-prepared"
		},
		ExecuteAttempt: func(_ context.Context, in RemoteSubmitInput[string, string, string, string]) RemoteSubmitResult[string, string] {
			if in.TaskID != "task-1" || in.ProductAPI != "api" {
				t.Fatalf("unexpected submit input")
			}
			return RemoteSubmitResult[string, string]{
				Response: "response",
				Snapshot: "snapshot-attempt",
				Err:      expectedErr,
			}
		},
	})

	result := service.Submit(context.Background(), RemoteSubmitInput[string, string, string, string]{
		TaskID:     "task-1",
		Package:    "pkg",
		Action:     "publish",
		RequestID:  "req-1",
		ProductAPI: "api",
		Product:    "product",
		Snapshot:   "snapshot-in",
	})

	if result.SupplierCode != "SUP-1" {
		t.Fatalf("supplier code = %q, want SUP-1", result.SupplierCode)
	}
	if result.Response != "response" {
		t.Fatalf("response = %q, want response", result.Response)
	}
	if result.Snapshot != "snapshot-attempt" {
		t.Fatalf("snapshot = %q, want snapshot-attempt", result.Snapshot)
	}
	if !errors.Is(result.Err, expectedErr) {
		t.Fatalf("err = %v, want %v", result.Err, expectedErr)
	}
}
