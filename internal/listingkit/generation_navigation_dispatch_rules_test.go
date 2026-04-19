package listingkit

import "testing"

func TestApplyGenerationNavigationDispatchExecutionRulesMarksFallbackSessionWinner(t *testing.T) {
	t.Parallel()

	plan := &GenerationNavigationDispatchPlan{
		Strategy:         "fanout_read",
		FallbackStrategy: "prefer_preview_then_session_then_queue",
	}
	execution := &GenerationNavigationDispatchExecution{
		Steps: []GenerationNavigationDispatchExecutionStep{
			{
				Kind:      "preview",
				Status:    "failed",
				Error:     "preview missing",
				ErrorKind: "not_found",
			},
			{
				Kind:      "queue",
				Status:    "failed",
				Error:     "queue unavailable",
				ErrorKind: "internal",
			},
			{
				Kind:   "session",
				Status: "completed",
				ReviewSession: &GenerationReviewSessionResponse{
					Session: &GenerationReviewSession{
						Queue: &GenerationWorkQueue{
							Summary: &GenerationWorkQueueSummary{TotalItems: 1},
						},
					},
				},
			},
		},
	}

	applyGenerationNavigationDispatchExecutionRules(plan, execution)

	sessionWinner := bestGenerationNavigationDispatchExecutionStep(execution, "session")
	if sessionWinner == nil || !sessionWinner.Winner {
		t.Fatalf("session winner = %+v, want winner session step", sessionWinner)
	}
	if !sessionWinner.FallbackApplied || sessionWinner.FallbackReason == "" {
		t.Fatalf("session winner = %+v, want fallback applied", sessionWinner)
	}
	if !sessionWinner.FallbackCandidate || sessionWinner.FallbackSourceKind != "session" {
		t.Fatalf("session winner = %+v, want session fallback metadata", sessionWinner)
	}
	queueStep := &execution.Steps[1]
	if queueStep.Retryable || queueStep.RetryHint != "review_fallback" || !queueStep.FallbackCandidate || queueStep.FallbackSourceKind != "session" {
		t.Fatalf("queue step = %+v, want review fallback queue failure", queueStep)
	}
	previewStep := &execution.Steps[0]
	if previewStep.Retryable || previewStep.RetryHint != "review_fallback" || !previewStep.FallbackCandidate || previewStep.FallbackSourceKind != "session" {
		t.Fatalf("preview step = %+v, want review fallback preview failure", previewStep)
	}
}

func TestApplyGenerationNavigationDispatchExecutionRulesMarksInternalFailureRetryableWithoutFallback(t *testing.T) {
	t.Parallel()

	execution := &GenerationNavigationDispatchExecution{
		Steps: []GenerationNavigationDispatchExecutionStep{{
			Kind:      "preview",
			Status:    "failed",
			Error:     "boom",
			ErrorKind: "internal",
		}},
	}

	applyGenerationNavigationDispatchExecutionRules(&GenerationNavigationDispatchPlan{}, execution)

	step := execution.Steps[0]
	if !step.Retryable || step.RetryHint != "retry_dispatch" {
		t.Fatalf("step = %+v, want retryable internal failure without fallback", step)
	}
}

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
