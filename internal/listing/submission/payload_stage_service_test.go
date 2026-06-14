package submission

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPayloadStageServicePrepareUploadAndPreValidate(t *testing.T) {
	t.Parallel()

	var phases []string
	var calls []string
	svc := NewPayloadStageService(PayloadStageServiceConfig[string, string, string, string]{
		Phases: PayloadStagePhases{
			PrepareProduct: "prepare",
			UploadImages:   "upload",
			PreValidate:    "validate",
		},
		PersistPhase: func(_ context.Context, _ PayloadStageContext[string, string], phase string) error {
			phases = append(phases, phase)
			return nil
		},
		PreparePayload: func(context.Context, PayloadStageContext[string, string]) (*PreparedPayload[string, string], error) {
			calls = append(calls, "prepare")
			return &PreparedPayload[string, string]{
				Product:          "product",
				NeedsImageUpload: true,
				Snapshot:         "snapshot-prepared",
			}, nil
		},
		PersistSnapshot: func(_ context.Context, _ PayloadStageContext[string, string], snapshot string) error {
			calls = append(calls, "snapshot:"+snapshot)
			return nil
		},
		RequirePreparedPayload: func(payload *PreparedPayload[string, string]) error {
			if payload == nil || payload.Product == "" {
				return errors.New("payload required")
			}
			return nil
		},
		UploadImages: func(context.Context, PayloadStageContext[string, string], string) error {
			calls = append(calls, "upload")
			return nil
		},
		FinalizeUploaded: func(context.Context, PayloadStageContext[string, string], *PreparedPayload[string, string]) (*PreparedPayload[string, string], error) {
			calls = append(calls, "finalize")
			return &PreparedPayload[string, string]{
				Product:          "product",
				NeedsImageUpload: false,
				Snapshot:         "snapshot-uploaded",
			}, nil
		},
		PreValidate: func(context.Context, PayloadStageContext[string, string], string) error {
			calls = append(calls, "prevalidate")
			return nil
		},
	})

	in := PayloadStageContext[string, string]{TaskID: "task-1"}
	prepared, err := svc.Prepare(context.Background(), in)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	uploaded, err := svc.UploadImages(context.Background(), in, prepared)
	if err != nil {
		t.Fatalf("UploadImages() error = %v", err)
	}
	if err := svc.PreValidate(context.Background(), in, uploaded); err != nil {
		t.Fatalf("PreValidate() error = %v", err)
	}

	if !reflect.DeepEqual(phases, []string{"prepare", "upload", "validate"}) {
		t.Fatalf("phases = %v", phases)
	}
	if !reflect.DeepEqual(calls, []string{
		"prepare",
		"snapshot:snapshot-prepared",
		"upload",
		"finalize",
		"snapshot:snapshot-uploaded",
		"prevalidate",
	}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestPayloadStageServiceUploadSkipsWhenImagesNotNeeded(t *testing.T) {
	t.Parallel()

	var phases []string
	svc := NewPayloadStageService(PayloadStageServiceConfig[string, string, string, string]{
		Phases: PayloadStagePhases{
			PrepareProduct: "prepare",
			UploadImages:   "upload",
			PreValidate:    "validate",
		},
		PersistPhase: func(_ context.Context, _ PayloadStageContext[string, string], phase string) error {
			phases = append(phases, phase)
			return nil
		},
		RequirePreparedPayload: func(*PreparedPayload[string, string]) error { return nil },
		UploadImages:           func(context.Context, PayloadStageContext[string, string], string) error { return nil },
		FinalizeUploaded: func(context.Context, PayloadStageContext[string, string], *PreparedPayload[string, string]) (*PreparedPayload[string, string], error) {
			t.Fatal("FinalizeUploaded should not be called")
			return nil, nil
		},
	})

	payload := &PreparedPayload[string, string]{Product: "product", NeedsImageUpload: false}
	out, err := svc.UploadImages(context.Background(), PayloadStageContext[string, string]{TaskID: "task-1"}, payload)
	if err != nil {
		t.Fatalf("UploadImages() error = %v", err)
	}
	if out != payload {
		t.Fatalf("payload pointer changed = %+v", out)
	}
	if len(phases) != 0 {
		t.Fatalf("phases = %v, want none", phases)
	}
}

func TestPayloadStageServiceReturnsPreparedPayloadValidationError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("payload required")
	svc := NewPayloadStageService(PayloadStageServiceConfig[string, string, string, string]{
		Phases: PayloadStagePhases{UploadImages: "upload", PreValidate: "validate"},
		RequirePreparedPayload: func(*PreparedPayload[string, string]) error {
			return expectedErr
		},
	})

	if _, err := svc.UploadImages(context.Background(), PayloadStageContext[string, string]{TaskID: "task-1"}, nil); !errors.Is(err, expectedErr) {
		t.Fatalf("UploadImages() error = %v, want %v", err, expectedErr)
	}
	if err := svc.PreValidate(context.Background(), PayloadStageContext[string, string]{TaskID: "task-1"}, nil); !errors.Is(err, expectedErr) {
		t.Fatalf("PreValidate() error = %v, want %v", err, expectedErr)
	}
}
