package listingkit

import "testing"

func TestApplyGenerationNavigationDispatchExecutionMergePrefersWinnerSteps(t *testing.T) {
	t.Parallel()

	response := &GenerationReviewNavigationDispatchResponse{}
	execution := &GenerationNavigationDispatchExecution{
		Steps: []GenerationNavigationDispatchExecutionStep{
			{
				Kind:       "queue",
				Status:     "completed",
				DeltaToken: "queue-token",
				Queue:      &GenerationQueuePage{TaskID: "task-1"},
			},
			{
				Kind:       "session",
				Status:     "completed",
				DeltaToken: "session-token",
				ReviewSession: &GenerationReviewSessionResponse{
					TaskID: "task-1",
				},
			},
			{
				Kind:       "preview",
				Status:     "not_modified",
				DeltaToken: "preview-token",
				ReviewPreview: &GenerationReviewPreviewResponse{
					TaskID: "task-1",
				},
			},
		},
	}

	applyGenerationNavigationDispatchExecutionRules(&GenerationNavigationDispatchPlan{}, execution)
	applyGenerationNavigationDispatchExecutionMerge(response, execution)

	if response.Queue == nil || response.ReviewSession == nil || response.ReviewPreview == nil {
		t.Fatalf("response = %+v, want merged queue/session/preview winners", response)
	}
	if response.DeltaToken != "preview-token" {
		t.Fatalf("response delta token = %q, want preview winner token", response.DeltaToken)
	}
	if response.FocusedSourceKind != "preview" || response.FocusedSourceStep != 2 || response.FocusedViaFallback {
		t.Fatalf("response focused source = %+v, want preview winner source", response)
	}
	if response.FocusedResolution == nil || response.FocusedResolution.SourceKind != "preview" || response.FocusedResolution.SourceStep != 2 {
		t.Fatalf("response focused resolution = %+v, want preview resolution", response.FocusedResolution)
	}
}
