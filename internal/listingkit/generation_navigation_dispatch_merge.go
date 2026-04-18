package listingkit

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

func firstStepDeltaToken(step *GenerationNavigationDispatchExecutionStep) string {
	if step == nil {
		return ""
	}
	return step.DeltaToken
}
