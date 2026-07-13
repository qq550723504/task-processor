package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
)

func appendStudioGenerationRouteDescriptors(routes []httproute.Descriptor, handler StudioGenerationRouteHandler) []httproute.Descriptor {
	routes = append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/reference-style/analyze", Module: "listing-kit", Handler: handler.AnalyzeStudioReferenceStyle},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/designs", Module: "listing-kit", Handler: handler.GenerateStudioDesigns},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/product-images", Module: "listing-kit", Handler: handler.GenerateStudioProductImages},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/async-jobs", Module: "listing-kit", Handler: handler.StartStudioAsyncJob},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/async-jobs/:job_id", Module: "listing-kit", Handler: handler.GetStudioAsyncJob},
	)
	batchRuns, ok := handler.(studioBatchRunRouteHandler)
	if !ok {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batch-runs", Module: "listing-kit", Handler: batchRuns.CreateStudioBatchRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id", Module: "listing-kit", Handler: batchRuns.GetStudioBatchRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id/items", Module: "listing-kit", Handler: batchRuns.ListStudioBatchRunItems},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id/cancel", Module: "listing-kit", Handler: batchRuns.CancelStudioBatchRun},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batch-runs/:run_id/recover", Module: "listing-kit", Handler: batchRuns.RecoverStudioBatchRun},
	)
}

func appendTaskRouteDescriptors(routes []httproute.Descriptor, handler TaskRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein-images/regenerate", Module: "listing-kit", Handler: handler.RegenerateSheinDataImage},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/uploads/images", Module: "listing-kit", Handler: handler.UploadListingKitImages},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.GetUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/uploads/files/*key", Module: "listing-kit", Handler: handler.DeleteUploadedListingKitImage},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks", Module: "listing-kit", Handler: handler.ListTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/sds/baselines/readiness", Module: "listing-kit", Handler: handler.GetSDSBaselineReadiness},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/sds/baselines/warm", Module: "listing-kit", Handler: handler.WarmSDSBaseline},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/sds/retirements", Module: "listing-kit", Handler: handler.CreateSDSRetirementRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/sds/retirements/:run_id", Module: "listing-kit", Handler: handler.GetSDSRetirementRun},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/sds/retirements/:run_id/items", Module: "listing-kit", Handler: handler.UpdateSDSRetirementSelection},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/sds/retirements/:run_id/confirm", Module: "listing-kit", Handler: handler.ConfirmSDSRetirementRun},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/sds/retirements/:run_id/retry", Module: "listing-kit", Handler: handler.RetrySDSRetirementRun},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id", Module: "listing-kit", Handler: handler.GetTaskResult},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/requeue", Module: "listing-kit", Handler: handler.RequeuePendingTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/recover", Module: "listing-kit", Handler: handler.RecoverTaskNow},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/recovery/recover", Module: "listing-kit", Handler: handler.BulkRecoverTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/preview", Module: "listing-kit", Handler: handler.GetTaskPreview},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks", Module: "listing-kit", Handler: handler.GetTaskGenerationTasks},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-queue", Module: "listing-kit", Handler: handler.GetTaskGenerationQueue},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-session", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewSession},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/generation-review-preview", Module: "listing-kit", Handler: handler.GetTaskGenerationReviewPreview},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", Module: "listing-kit", Handler: handler.DispatchTaskGenerationNavigation},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", Module: "listing-kit", Handler: handler.RetryTaskGenerationTasks},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", Module: "listing-kit", Handler: handler.RetryTaskChildTask},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/sds-repair", Module: "listing-kit", Handler: handler.GetTaskSDSRepair},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/sds-repair/retry", Module: "listing-kit", Handler: handler.RepairAndRetryTaskSDS},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", Module: "listing-kit", Handler: handler.ExecuteTaskGenerationAction},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history", Module: "listing-kit", Handler: handler.GetTaskRevisionHistory},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", Module: "listing-kit", Handler: handler.GetTaskRevisionHistoryDetail},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/export", Module: "listing-kit", Handler: handler.GetTaskExport},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision", Module: "listing-kit", Handler: handler.ApplyTaskRevision},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/revision/validate", Module: "listing-kit", Handler: handler.ValidateTaskRevision},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/shein/price-preview", Module: "listing-kit", Handler: handler.PreviewSheinPrice},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/shein/categories", Module: "listing-kit", Handler: handler.SearchSheinCategories},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/tasks/:task_id/shein/final-draft", Module: "listing-kit", Handler: handler.UpdateSheinFinalDraft},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/submission-events", Module: "listing-kit", Handler: handler.GetSubmissionEvents},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submit", Module: "listing-kit", Handler: handler.SubmitTask},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/submission-status/refresh", Module: "listing-kit", Handler: handler.RefreshSubmissionStatus},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/listing-kits/tasks/:task_id/shein-resolution-cache", Module: "listing-kit", Handler: handler.ClearSheinResolutionCache},
	)
}
