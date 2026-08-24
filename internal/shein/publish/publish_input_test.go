package publish

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

func newMappingRequestTaskContext(store *listingruntime.StoreInfo) *shein.TaskContext {
	ctx := shein.NewTaskContext(context.Background(), &model.Task{ID: 1})
	ctx.SetStoreInfo(store)
	return ctx
}

func TestBuildMappingRequestInputRejectsMissingStoreOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		store *listingruntime.StoreInfo
		want  string
	}{
		{name: "missing store", want: "store info is not initialized"},
		{name: "blank owner", store: &listingruntime.StoreInfo{OwnerUserID: "  "}, want: "store owner is not initialized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildMappingRequestInput(newMappingRequestTaskContext(tt.store))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildMappingRequestInputPreservesStoreOwner(t *testing.T) {
	t.Parallel()

	input, err := buildMappingRequestInput(newMappingRequestTaskContext(
		&listingruntime.StoreInfo{OwnerUserID: "zitadel-sub-1"},
	))
	if err != nil {
		t.Fatalf("buildMappingRequestInput() error = %v", err)
	}
	if input.StoreInfo == nil || input.StoreInfo.OwnerUserID != "zitadel-sub-1" {
		t.Fatalf("input.StoreInfo = %+v, want owner zitadel-sub-1", input.StoreInfo)
	}
}
