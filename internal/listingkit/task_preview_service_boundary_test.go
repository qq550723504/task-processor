package listingkit

import "testing"

func TestTaskPreviewServiceDelegatesThroughPreviewDomainService(t *testing.T) {
	t.Parallel()

	constructorSource := readNamedFunctionSource(t, "task_preview_service.go", "newTaskPreviewService")
	getSource := readNamedFunctionSource(t, "task_preview_service.go", "GetTaskPreview")
	constructorCalls := readNamedFunctionCallNames(t, "task_preview_service.go", "newTaskPreviewService")
	getCalls := readNamedFunctionCallNames(t, "task_preview_service.go", "GetTaskPreview")

	assertSourceContainsAll(t, constructorSource, []string{
		"svc.reader = previewdomain.NewTaskPreviewService",
		"BuildPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"FinalizePreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) error {",
	})
	assertFunctionCallsContainAll(t, constructorCalls, []string{
		"NewTaskPreviewService",
		"buildTaskPreview",
		"finalizeTaskPreview",
	})
	assertSourceContainsAll(t, getSource, []string{
		"return s.reader.GetTaskPreview(ctx, taskID, platform)",
	})
	assertFunctionCallsContainAll(t, getCalls, []string{
		"GetTaskPreview",
	})
}
