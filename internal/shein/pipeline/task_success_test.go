package pipeline

import (
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheincontext "task-processor/internal/shein/context"
)

func TestSuccessStatusForTaskContext(t *testing.T) {
	enableDraft := true
	disableDraft := false

	tests := []struct {
		name string
		ctx  *sheincontext.TaskContext
		want model.TaskStatus
	}{
		{
			name: "draft enabled store completes as draft",
			ctx:  &sheincontext.TaskContext{RuntimeState: sheincontext.RuntimeState{StoreInfo: &listingruntime.StoreInfo{EnableDraft: &enableDraft}}},
			want: model.TaskStatusDraft,
		},
		{
			name: "draft disabled store completes as published",
			ctx:  &sheincontext.TaskContext{RuntimeState: sheincontext.RuntimeState{StoreInfo: &listingruntime.StoreInfo{EnableDraft: &disableDraft}}},
			want: model.TaskStatusPublished,
		},
		{
			name: "missing draft setting completes as published",
			ctx:  &sheincontext.TaskContext{RuntimeState: sheincontext.RuntimeState{StoreInfo: &listingruntime.StoreInfo{}}},
			want: model.TaskStatusPublished,
		},
		{
			name: "missing store completes as published",
			ctx:  &sheincontext.TaskContext{},
			want: model.TaskStatusPublished,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := successStatusForTaskContext(tt.ctx); got != tt.want {
				t.Fatalf("successStatusForTaskContext() = %s, want %s", got, tt.want)
			}
		})
	}
}
