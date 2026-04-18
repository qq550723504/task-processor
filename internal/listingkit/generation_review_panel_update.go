package listingkit

func buildGenerationReviewPanelUpdateFromDispatch(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewPanelUpdate {
	if response == nil {
		return nil
	}
	update := &GenerationReviewPanelUpdate{
		DispatchKind: response.DispatchKind,
		ResponseMode: normalizeGenerationActionResponseMode(response.ResponseMode),
		DeltaToken:   response.DeltaToken,
	}
	switch response.DispatchKind {
	case "action":
		if !isPatchOnlyGenerationPanelUpdate(update.ResponseMode) {
			update.Action = response.Action
		}
		if response.Action != nil {
			update.DeltaToken = firstNonEmpty(response.Action.DeltaToken, update.DeltaToken)
			update.Overview = response.Action.Overview
			update.ReviewPatch = response.Action.ReviewPatch
			if response.Action.ReviewPatch != nil {
				update.QueueSummary = response.Action.ReviewPatch.QueueSummary
				update.ReviewSummary = response.Action.ReviewPatch.ReviewSummary
				update.FocusedTarget = response.Action.ReviewPatch.FocusedTarget
				update.FocusedRenderPreview = response.Action.ReviewPatch.FocusedRenderPreview
				update.FocusedToolbar = response.Action.ReviewPatch.FocusedToolbar
			}
			if response.Action.ReviewSession != nil {
				mergeGenerationReviewSessionIntoPanelUpdate(update, response.Action.ReviewSession)
			}
		}
	case "preview":
		if !isPatchOnlyGenerationPanelUpdate(update.ResponseMode) {
			update.ReviewPreview = response.ReviewPreview
		}
		if response.ReviewPreview != nil {
			update.DeltaToken = firstNonEmpty(response.ReviewPreview.DeltaToken, update.DeltaToken)
			if response.ReviewPreview.NotModified {
				update.NoChanges = true
				break
			}
			update.FocusedTarget = response.ReviewPreview.ReviewTarget
			update.FocusedRenderPreview = response.ReviewPreview.Preview
			update.FocusedToolbar = response.ReviewPreview.Toolbar
		}
	case "session":
		if !isPatchOnlyGenerationPanelUpdate(update.ResponseMode) {
			update.ReviewSession = response.ReviewSession
		}
		if response.ReviewSession != nil {
			update.DeltaToken = firstNonEmpty(response.ReviewSession.DeltaToken, update.DeltaToken)
			if response.ReviewSession.Session != nil {
				mergeGenerationReviewSessionIntoPanelUpdate(update, response.ReviewSession.Session)
			}
			if response.ReviewSession.Patch != nil {
				update.ReviewPatch = response.ReviewSession.Patch
				update.Overview = firstOverview(update.Overview, response.ReviewSession.Patch.Overview)
				update.QueueSummary = firstQueueSummary(update.QueueSummary, response.ReviewSession.Patch.QueueSummary)
				update.ReviewSummary = firstReviewSummary(update.ReviewSummary, response.ReviewSession.Patch.ReviewSummary)
				update.FocusedTarget = firstFocusedTarget(update.FocusedTarget, response.ReviewSession.Patch.FocusedTarget)
				update.FocusedRenderPreview = firstRenderPreview(update.FocusedRenderPreview, response.ReviewSession.Patch.FocusedRenderPreview)
				update.FocusedToolbar = firstToolbar(update.FocusedToolbar, response.ReviewSession.Patch.FocusedToolbar)
			}
		}
	case "queue":
		if response.Queue != nil {
			update.DeltaToken = firstNonEmpty(response.Queue.DeltaToken, update.DeltaToken)
			if response.Queue.NotModified {
				update.NoChanges = true
				break
			}
			update.QueueSummary = response.Queue.Summary
		}
	}
	mergeSupplementalGenerationDispatchResultsIntoPanelUpdate(update, response)
	mergeGenerationDispatchFocusedSourceIntoPanelUpdate(update, response)
	if isPatchOnlyGenerationPanelUpdate(update.ResponseMode) {
		minimizeGenerationReviewPanelUpdate(update)
	}
	if update.NoChanges || isGenerationReviewPanelUpdateEmpty(update) {
		update.NoChanges = true
		update.ReviewPatch = nil
	}
	return applyGenerationConditionalStateToPanelUpdate(update)
}

func mergeGenerationDispatchFocusedSourceIntoPanelUpdate(update *GenerationReviewPanelUpdate, response *GenerationReviewNavigationDispatchResponse) {
	if update == nil || response == nil {
		return
	}
	update.FocusedSourceKind = firstNonEmpty(update.FocusedSourceKind, response.FocusedSourceKind)
	if response.FocusedSourceKind != "" && update.FocusedSourceKind == response.FocusedSourceKind {
		update.FocusedSourceStep = response.FocusedSourceStep
	}
	if response.FocusedViaFallback {
		update.FocusedViaFallback = true
	}
	update.FocusedFallbackReason = firstNonEmpty(update.FocusedFallbackReason, response.FocusedFallbackReason)
	if update.FocusedResolution == nil && response.FocusedResolution != nil {
		cloned := *response.FocusedResolution
		update.FocusedResolution = &cloned
	}
}

func mergeSupplementalGenerationDispatchResultsIntoPanelUpdate(update *GenerationReviewPanelUpdate, response *GenerationReviewNavigationDispatchResponse) {
	if update == nil || response == nil {
		return
	}
	if response.Queue != nil && !response.Queue.NotModified {
		update.DeltaToken = firstNonEmpty(update.DeltaToken, response.Queue.DeltaToken)
		update.QueueSummary = firstQueueSummary(update.QueueSummary, response.Queue.Summary)
	}
	if response.ReviewSession != nil {
		update.DeltaToken = firstNonEmpty(update.DeltaToken, response.ReviewSession.DeltaToken)
		if response.ReviewSession.Session != nil {
			mergeGenerationReviewSessionIntoPanelUpdate(update, response.ReviewSession.Session)
		}
		if response.ReviewSession.Patch != nil {
			update.ReviewPatch = firstReviewPatch(update.ReviewPatch, response.ReviewSession.Patch)
			update.Overview = firstOverview(update.Overview, response.ReviewSession.Patch.Overview)
			update.QueueSummary = firstQueueSummary(update.QueueSummary, response.ReviewSession.Patch.QueueSummary)
			update.ReviewSummary = firstReviewSummary(update.ReviewSummary, response.ReviewSession.Patch.ReviewSummary)
			update.FocusedTarget = firstFocusedTarget(update.FocusedTarget, response.ReviewSession.Patch.FocusedTarget)
			update.FocusedRenderPreview = firstRenderPreview(update.FocusedRenderPreview, response.ReviewSession.Patch.FocusedRenderPreview)
			update.FocusedToolbar = firstToolbar(update.FocusedToolbar, response.ReviewSession.Patch.FocusedToolbar)
		}
	}
	if response.ReviewPreview != nil && !response.ReviewPreview.NotModified {
		update.DeltaToken = firstNonEmpty(update.DeltaToken, response.ReviewPreview.DeltaToken)
		update.FocusedTarget = firstFocusedTarget(update.FocusedTarget, response.ReviewPreview.ReviewTarget)
		update.FocusedRenderPreview = firstRenderPreview(update.FocusedRenderPreview, response.ReviewPreview.Preview)
		update.FocusedToolbar = firstToolbar(update.FocusedToolbar, response.ReviewPreview.Toolbar)
	}
}

func isPatchOnlyGenerationPanelUpdate(mode string) bool {
	return normalizeGenerationActionResponseMode(mode) == "patch_only"
}

func mergeGenerationReviewSessionIntoPanelUpdate(update *GenerationReviewPanelUpdate, session *GenerationReviewSession) {
	if update == nil || session == nil {
		return
	}
	update.Overview = firstOverview(update.Overview, session.Overview)
	if session.Queue != nil {
		update.QueueSummary = firstQueueSummary(update.QueueSummary, session.Queue.Summary)
	}
	update.ReviewSummary = firstReviewSummary(update.ReviewSummary, session.ReviewSummary)
	update.FocusedTarget = firstFocusedTarget(update.FocusedTarget, session.FocusedTarget)
	update.FocusedRenderPreview = firstRenderPreview(update.FocusedRenderPreview, session.FocusedRenderPreview)
	update.FocusedToolbar = firstToolbar(update.FocusedToolbar, session.FocusedToolbar)
}

func minimizeGenerationReviewPanelUpdate(update *GenerationReviewPanelUpdate) {
	if update == nil {
		return
	}
	if update.ReviewPatch != nil {
		update.Overview = nil
		update.QueueSummary = nil
		update.ReviewSummary = nil
		update.FocusedTarget = nil
		update.FocusedRenderPreview = nil
		update.FocusedToolbar = nil
	}
}

func isGenerationReviewPanelUpdateEmpty(update *GenerationReviewPanelUpdate) bool {
	if update == nil {
		return true
	}
	if update.Overview != nil ||
		update.QueueSummary != nil ||
		update.ReviewSummary != nil ||
		update.FocusedTarget != nil ||
		update.FocusedRenderPreview != nil ||
		update.FocusedToolbar != nil ||
		update.ReviewSession != nil ||
		update.ReviewPreview != nil ||
		update.Action != nil {
		return false
	}
	return isGenerationReviewSessionPatchEmpty(update.ReviewPatch)
}

func firstOverview(current, candidate *AssetGenerationOverview) *AssetGenerationOverview {
	if current != nil {
		return current
	}
	return candidate
}

func firstQueueSummary(current, candidate *GenerationWorkQueueSummary) *GenerationWorkQueueSummary {
	if current != nil {
		return current
	}
	return candidate
}

func firstReviewSummary(current, candidate *GenerationReviewSummary) *GenerationReviewSummary {
	if current != nil {
		return current
	}
	return candidate
}

func firstFocusedTarget(current, candidate *GenerationReviewTarget) *GenerationReviewTarget {
	if current != nil {
		return current
	}
	return candidate
}

func firstRenderPreview(current, candidate *AssetRenderPreviewSlot) *AssetRenderPreviewSlot {
	if current != nil {
		return current
	}
	return candidate
}

func firstToolbar(current, candidate *GenerationReviewToolbarInput) *GenerationReviewToolbarInput {
	if current != nil {
		return current
	}
	return candidate
}

func firstReviewPatch(current, candidate *GenerationReviewSessionPatch) *GenerationReviewSessionPatch {
	if current != nil {
		return current
	}
	return candidate
}
