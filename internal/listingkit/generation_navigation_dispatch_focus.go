package listingkit

type generationNavigationFocusedSource struct {
	Kind           string
	StepIndex      int
	ViaFallback    bool
	FallbackReason string
	Target         *GenerationReviewTarget
	Preview        *AssetRenderPreviewSlot
	Toolbar        *GenerationReviewToolbarInput
}

func buildGenerationNavigationFocusedSource(response *GenerationReviewNavigationDispatchResponse, execution *GenerationNavigationDispatchExecution) *generationNavigationFocusedSource {
	if execution == nil {
		return nil
	}
	if winner, index := bestGenerationNavigationDispatchExecutionStepWithIndex(execution, "preview"); winner != nil {
		preview := firstReviewPreviewResponse(firstReviewPreviewStep(winner), responseReviewPreview(response))
		return &generationNavigationFocusedSource{
			Kind:           "preview",
			StepIndex:      index,
			ViaFallback:    winner.FallbackApplied,
			FallbackReason: winner.FallbackReason,
			Target:         previewReviewTarget(preview),
			Preview:        previewSlot(preview),
			Toolbar:        previewToolbar(preview),
		}
	}
	if winner, index := bestGenerationNavigationDispatchExecutionStepWithIndex(execution, "session"); winner != nil {
		session := firstReviewSessionResponse(firstReviewSessionStep(winner), responseReviewSession(response))
		return &generationNavigationFocusedSource{
			Kind:           "session",
			StepIndex:      index,
			ViaFallback:    winner.FallbackApplied,
			FallbackReason: firstNonEmpty(winner.FallbackReason, "session_focus_fallback"),
			Target:         sessionFocusedTarget(session),
			Preview:        sessionFocusedPreview(session),
			Toolbar:        sessionFocusedToolbar(session),
		}
	}
	return nil
}

func bestGenerationNavigationDispatchExecutionStepWithIndex(execution *GenerationNavigationDispatchExecution, kind string) (*GenerationNavigationDispatchExecutionStep, int) {
	if execution == nil {
		return nil, -1
	}
	bestScore := -1
	bestIndex := -1
	for index := range execution.Steps {
		step := &execution.Steps[index]
		if step.Kind != kind {
			continue
		}
		score := generationNavigationDispatchStepWinnerScore(step)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestIndex < 0 || bestScore <= 0 {
		return nil, -1
	}
	return &execution.Steps[bestIndex], bestIndex
}

func firstReviewPreviewResponse(candidates ...*GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	for _, candidate := range candidates {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func firstReviewSessionResponse(candidates ...*GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	for _, candidate := range candidates {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func firstReviewPreviewStep(step *GenerationNavigationDispatchExecutionStep) *GenerationReviewPreviewResponse {
	if step == nil {
		return nil
	}
	return step.ReviewPreview
}

func firstReviewSessionStep(step *GenerationNavigationDispatchExecutionStep) *GenerationReviewSessionResponse {
	if step == nil {
		return nil
	}
	return step.ReviewSession
}

func responseReviewPreview(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	return response.ReviewPreview
}

func responseReviewSession(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	return response.ReviewSession
}

func previewReviewTarget(response *GenerationReviewPreviewResponse) *GenerationReviewTarget {
	if response == nil {
		return nil
	}
	return response.ReviewTarget
}

func previewSlot(response *GenerationReviewPreviewResponse) *AssetRenderPreviewSlot {
	if response == nil {
		return nil
	}
	return response.Preview
}

func previewToolbar(response *GenerationReviewPreviewResponse) *GenerationReviewToolbarInput {
	if response == nil {
		return nil
	}
	return response.Toolbar
}

func sessionFocusedTarget(response *GenerationReviewSessionResponse) *GenerationReviewTarget {
	if response == nil {
		return nil
	}
	if response.Session != nil && response.Session.FocusedTarget != nil {
		return response.Session.FocusedTarget
	}
	if response.Patch != nil && response.Patch.FocusedTarget != nil {
		return response.Patch.FocusedTarget
	}
	return nil
}

func sessionFocusedPreview(response *GenerationReviewSessionResponse) *AssetRenderPreviewSlot {
	if response == nil {
		return nil
	}
	if response.Session != nil && response.Session.FocusedRenderPreview != nil {
		return response.Session.FocusedRenderPreview
	}
	if response.Patch != nil && response.Patch.FocusedRenderPreview != nil {
		return response.Patch.FocusedRenderPreview
	}
	return nil
}

func sessionFocusedToolbar(response *GenerationReviewSessionResponse) *GenerationReviewToolbarInput {
	if response == nil {
		return nil
	}
	if response.Session != nil && response.Session.FocusedToolbar != nil {
		return response.Session.FocusedToolbar
	}
	if response.Patch != nil && response.Patch.FocusedToolbar != nil {
		return response.Patch.FocusedToolbar
	}
	return nil
}
