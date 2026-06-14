package submission

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestDirectSubmitFlowServiceSubmit(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var calls []string
	var phases []string

	service := NewDirectSubmitFlowService(DirectSubmitFlowServiceConfig[string, string, string, string, string]{
		Phases: DirectSubmitFlowPhases{
			PrepareProduct: "prepare",
			UploadImages:   "upload",
			PreValidate:    "validate",
			SubmitRemote:   "remote",
		},
		BuildProductAPI: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			calls = append(calls, "build_api")
			return "api", nil
		},
		PersistPhase: func(_ context.Context, _ string, _ string, _ string, _ DirectSubmitFlowOptions, phase string) error {
			calls = append(calls, "persist")
			phases = append(phases, phase)
			return nil
		},
		PrepareProduct: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			calls = append(calls, "prepare_product")
			return "product", nil
		},
		NeedsImageUpload: func(product string) bool {
			return product == "product"
		},
		UploadImages: func(context.Context, string, string, string, string, DirectSubmitFlowOptions) error {
			calls = append(calls, "upload_images")
			return nil
		},
		PreValidate: func(context.Context, string, string, string, string, DirectSubmitFlowOptions) error {
			calls = append(calls, "pre_validate")
			return nil
		},
		SubmitRemote: func(context.Context, string, string, string, string, string, DirectSubmitFlowOptions) error {
			calls = append(calls, "submit_remote")
			return nil
		},
		BuildTaskPreview: func(context.Context, string, string) (string, error) {
			calls = append(calls, "build_preview")
			return "preview", nil
		},
	})

	preview, err := service.Submit(context.Background(), DirectSubmitFlowInput[string, string]{
		TaskID:   "task-1",
		Task:     "task",
		Package:  "pkg",
		Platform: "shein",
		Options: DirectSubmitFlowOptions{
			Action:    "publish",
			RequestID: "req-1",
			StartedAt: now,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if preview != "preview" {
		t.Fatalf("preview = %q, want preview", preview)
	}
	if !reflect.DeepEqual(phases, []string{"prepare", "upload", "validate", "remote"}) {
		t.Fatalf("phases = %v", phases)
	}
	if !reflect.DeepEqual(calls, []string{
		"build_api",
		"persist",
		"prepare_product",
		"persist",
		"upload_images",
		"persist",
		"pre_validate",
		"persist",
		"submit_remote",
		"build_preview",
	}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestDirectSubmitFlowServiceSkipsUploadPhaseWhenNotNeeded(t *testing.T) {
	t.Parallel()

	var phases []string
	service := NewDirectSubmitFlowService(DirectSubmitFlowServiceConfig[string, string, string, string, string]{
		Phases: DirectSubmitFlowPhases{
			PrepareProduct: "prepare",
			UploadImages:   "upload",
			PreValidate:    "validate",
			SubmitRemote:   "remote",
		},
		BuildProductAPI: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			return "api", nil
		},
		PersistPhase: func(_ context.Context, _ string, _ string, _ string, _ DirectSubmitFlowOptions, phase string) error {
			phases = append(phases, phase)
			return nil
		},
		PrepareProduct: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			return "product", nil
		},
		NeedsImageUpload: func(string) bool { return false },
		PreValidate:      func(context.Context, string, string, string, string, DirectSubmitFlowOptions) error { return nil },
		SubmitRemote: func(context.Context, string, string, string, string, string, DirectSubmitFlowOptions) error {
			return nil
		},
		BuildTaskPreview: func(context.Context, string, string) (string, error) { return "preview", nil },
	})

	if _, err := service.Submit(context.Background(), DirectSubmitFlowInput[string, string]{TaskID: "task-1"}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if !reflect.DeepEqual(phases, []string{"prepare", "validate", "remote"}) {
		t.Fatalf("phases = %v", phases)
	}
}

func TestDirectSubmitFlowServiceReturnsCallbackErrors(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("submit remote failed")
	service := NewDirectSubmitFlowService(DirectSubmitFlowServiceConfig[string, string, string, string, string]{
		Phases: DirectSubmitFlowPhases{
			PrepareProduct: "prepare",
			PreValidate:    "validate",
			SubmitRemote:   "remote",
		},
		BuildProductAPI: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			return "api", nil
		},
		PersistPhase: func(context.Context, string, string, string, DirectSubmitFlowOptions, string) error { return nil },
		PrepareProduct: func(context.Context, string, string, string, DirectSubmitFlowOptions) (string, error) {
			return "product", nil
		},
		PreValidate: func(context.Context, string, string, string, string, DirectSubmitFlowOptions) error { return nil },
		SubmitRemote: func(context.Context, string, string, string, string, string, DirectSubmitFlowOptions) error {
			return expectedErr
		},
		BuildTaskPreview: func(context.Context, string, string) (string, error) { return "preview", nil },
	})

	_, err := service.Submit(context.Background(), DirectSubmitFlowInput[string, string]{TaskID: "task-1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Submit() error = %v, want %v", err, expectedErr)
	}
}
