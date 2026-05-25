package listingkit

import (
	"context"
	"testing"
)

func TestTaskTemporalSubmissionAdapterUploadSheinPublishImagesReturnsInputWhenUploadNotNeeded(t *testing.T) {
	t.Parallel()

	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{})
	input := &SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		Product:          makeReadySheinTask().Result.Shein.PreviewProduct,
		NeedsImageUpload: false,
	}

	out, err := adapter.UploadSheinPublishImages(context.Background(), input)
	if err != nil {
		t.Fatalf("UploadSheinPublishImages() error = %v", err)
	}
	if out != input {
		t.Fatalf("UploadSheinPublishImages() returned different payload pointer")
	}
}
