package product

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

func newPublishResultContext(store *listingadmin.StoreRespDTO) *temucontext.TemuTaskContext {
	ctx := temucontext.NewTemuTaskContext(context.Background(), &model.Task{ID: 1, TenantID: 2, StoreID: 3})
	ctx.SetSubmitResponse(&temuapi.SubmitResponse{Success: true})
	ctx.StoreInfo = store
	return ctx
}

func TestSavePublishResultHandlerLogsInputValidationError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	log := logrus.New()
	log.SetOutput(&output)
	handler := NewSavePublishResultHandler(nil, nil)
	handler.logger = logrus.NewEntry(log)

	if err := handler.HandleTemu(newPublishResultContext(nil)); err != nil {
		t.Fatalf("HandleTemu() error = %v", err)
	}
	if !strings.Contains(output.String(), "store info is not initialized") {
		t.Fatalf("log output = %q, want validation error", output.String())
	}
}

func TestBuildSavePublishResultInputRejectsMissingStoreOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		store *listingadmin.StoreRespDTO
		want  string
	}{
		{name: "missing store", want: "store info is not initialized"},
		{name: "blank owner", store: &listingadmin.StoreRespDTO{OwnerUserID: "  "}, want: "store owner is not initialized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSavePublishResultInput(newPublishResultContext(tt.store))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
