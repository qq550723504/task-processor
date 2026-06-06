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

func applyGenerationNavigationDispatchExecutionMerge(response *GenerationReviewNavigationDispatchResponse, execution *GenerationNavigationDispatchExecution) {
	if response == nil || execution == nil {
		return
	}
	response.ExecutedPlan = execution

	queueWinner := bestGenerationNavigationDispatchExecutionStep(execution, "queue")
	sessionWinner := bestGenerationNavigationDispatchExecutionStep(execution, "session")
	previewWinner := bestGenerationNavigationDispatchExecutionStep(execution, "preview")

	if queueWinner != nil && response.Queue == nil {
		response.Queue = queueWinner.Queue
	}
	if sessionWinner != nil && response.ReviewSession == nil {
		response.ReviewSession = sessionWinner.ReviewSession
	}
	if previewWinner != nil && response.ReviewPreview == nil {
		response.ReviewPreview = previewWinner.ReviewPreview
	}

	if previewWinner == nil && sessionWinner != nil && response.ReviewPreview == nil {
		response.DeltaToken = firstNonEmpty(response.DeltaToken, sessionWinner.DeltaToken)
	} else {
		response.DeltaToken = firstNonEmpty(
			response.DeltaToken,
			firstStepDeltaToken(previewWinner),
			firstStepDeltaToken(sessionWinner),
			firstStepDeltaToken(queueWinner),
		)
	}

	if focused := buildGenerationNavigationFocusedSource(response, execution); focused != nil {
		response.FocusedSourceKind = focused.Kind
		response.FocusedSourceStep = focused.StepIndex
		response.FocusedViaFallback = focused.ViaFallback
		response.FocusedFallbackReason = focused.FallbackReason
		response.FocusedResolution = buildGenerationNavigationDispatchResolution(focused)
		if focused.Target != nil {
			switch focused.Kind {
			case "preview":
				if response.ReviewPreview != nil && response.ReviewPreview.ReviewTarget == nil {
					response.ReviewPreview.ReviewTarget = focused.Target
				}
			case "session":
				if response.ReviewSession != nil {
					if response.ReviewSession.Session != nil && response.ReviewSession.Session.FocusedTarget == nil {
						response.ReviewSession.Session.FocusedTarget = focused.Target
					}
					if response.ReviewSession.Patch != nil && response.ReviewSession.Patch.FocusedTarget == nil {
						response.ReviewSession.Patch.FocusedTarget = focused.Target
					}
				}
			}
		}
		if focused.Preview != nil {
			switch focused.Kind {
			case "preview":
				if response.ReviewPreview != nil && response.ReviewPreview.Preview == nil {
					response.ReviewPreview.Preview = focused.Preview
				}
			case "session":
				if response.ReviewSession != nil {
					if response.ReviewSession.Session != nil && response.ReviewSession.Session.FocusedRenderPreview == nil {
						response.ReviewSession.Session.FocusedRenderPreview = focused.Preview
					}
					if response.ReviewSession.Patch != nil && response.ReviewSession.Patch.FocusedRenderPreview == nil {
						response.ReviewSession.Patch.FocusedRenderPreview = focused.Preview
					}
				}
			}
		}
		if focused.Toolbar != nil {
			switch focused.Kind {
			case "preview":
				if response.ReviewPreview != nil && response.ReviewPreview.Toolbar == nil {
					response.ReviewPreview.Toolbar = focused.Toolbar
				}
			case "session":
				if response.ReviewSession != nil {
					if response.ReviewSession.Session != nil && response.ReviewSession.Session.FocusedToolbar == nil {
						response.ReviewSession.Session.FocusedToolbar = focused.Toolbar
					}
					if response.ReviewSession.Patch != nil && response.ReviewSession.Patch.FocusedToolbar == nil {
						response.ReviewSession.Patch.FocusedToolbar = focused.Toolbar
					}
				}
			}
		}
	}
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

func buildGenerationNavigationDispatchResolution(focused *generationNavigationFocusedSource) *GenerationNavigationDispatchResolution {
	if focused == nil {
		return nil
	}
	return &GenerationNavigationDispatchResolution{
		SourceKind:     focused.Kind,
		SourceStep:     focused.StepIndex,
		ViaFallback:    focused.ViaFallback,
		FallbackReason: focused.FallbackReason,
	}
}

func firstStepDeltaToken(step *GenerationNavigationDispatchExecutionStep) string {
	if step == nil {
		return ""
	}
	return step.DeltaToken
}
