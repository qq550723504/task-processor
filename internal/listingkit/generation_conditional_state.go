package listingkit

import "strings"

func buildGenerationConditionalState(deltaToken string, notModified bool, noChanges bool) *GenerationConditionalState {
	token := strings.TrimSpace(deltaToken)
	if token == "" && !notModified && !noChanges {
		return nil
	}
	return &GenerationConditionalState{
		DeltaToken:  token,
		ETag:        buildGenerationConditionalETag(token),
		NotModified: notModified,
		NoChanges:   noChanges,
	}
}

func buildGenerationConditionalETag(deltaToken string) string {
	token := strings.TrimSpace(deltaToken)
	if token == "" {
		return ""
	}
	return `"` + token + `"`
}

func applyGenerationConditionalStateToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {
	if page == nil {
		return nil
	}
	page.Conditional = buildGenerationConditionalState(page.DeltaToken, page.NotModified, false)
	page.ResourceDescriptors = buildGenerationQueueResponseDescriptors(page)
	page = applyGenerationRecoverySummaryToQueuePage(page)
	return applyGenerationResolvedActionSummaryToQueuePage(page)
}

func applyGenerationConditionalStateToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	response.Conditional = buildGenerationConditionalState(response.DeltaToken, response.NotModified, false)
	applyConditionalStateToReviewSession(response.Session, response.Conditional)
	applyConditionalStateToReviewPatch(response.Patch, response.Conditional)
	response.ResourceDescriptors = buildGenerationSessionResponseDescriptors(response)
	response = applyGenerationRecoverySummaryToReviewSessionResponse(response)
	return applyGenerationResolvedActionSummaryToReviewSessionResponse(response)
}

func applyGenerationConditionalStateToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	response.Conditional = buildGenerationConditionalState(response.DeltaToken, response.NotModified, false)
	response.ReviewTarget = applyConditionalStateToReviewTarget(response.ReviewTarget, response.Conditional)
	response.Toolbar = applyConditionalStateToToolbarInput(response.Toolbar, response.Conditional)
	response.Viewer = applyConditionalStateToPreviewViewer(response.Viewer, response.Conditional)
	response.ResourceDescriptors = buildGenerationPreviewResponseDescriptors(response)
	response = applyGenerationRecoverySummaryToReviewPreviewResponse(response)
	return applyGenerationResolvedActionSummaryToReviewPreviewResponse(response)
}

func applyGenerationConditionalStateToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {
	if result == nil {
		return nil
	}
	result.Conditional = buildGenerationConditionalState(result.DeltaToken, false, false)
	if result.ResolvedTarget != nil {
		result.ResolvedTarget.NavigationTarget = applyConditionalStateToNavigationTarget(result.ResolvedTarget.NavigationTarget, result.Conditional)
	}
	applyConditionalStateToReviewSession(result.ReviewSession, result.Conditional)
	applyConditionalStateToReviewPatch(result.ReviewPatch, result.Conditional)
	result.ResourceDescriptors = buildGenerationActionResponseDescriptors(result)
	result = applyGenerationRecoverySummaryToActionResult(result)
	return applyGenerationResolvedActionSummaryToActionResult(result)
}

func applyGenerationConditionalStateToPanelUpdate(update *GenerationReviewPanelUpdate) *GenerationReviewPanelUpdate {
	if update == nil {
		return nil
	}
	update.Conditional = buildGenerationConditionalState(update.DeltaToken, false, update.NoChanges)
	update.FocusedTarget = applyConditionalStateToReviewTarget(update.FocusedTarget, update.Conditional)
	update.FocusedToolbar = applyConditionalStateToToolbarInput(update.FocusedToolbar, update.Conditional)
	applyConditionalStateToReviewPatch(update.ReviewPatch, update.Conditional)
	update.FocusedDescriptors = buildGenerationPanelFocusedDescriptors(update)
	update.ChangedDescriptors = buildGenerationPanelChangedDescriptors(update)
	update.PrimaryRecoveryDescriptor, update.RecommendedRecoveryDescriptors = buildGenerationPanelRecoverySelections(append(append([]GenerationPanelResourceDescriptor{}, update.FocusedDescriptors...), update.ChangedDescriptors...))
	return update
}

func applyGenerationConditionalStateToNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	response.Conditional = buildGenerationConditionalState(response.DeltaToken, response.NotModified, false)
	response.ResourceDescriptors = buildGenerationDispatchResponseDescriptors(response)
	response.PrimaryRecoveryDescriptor, response.RecommendedRecoveryDescriptors = buildGenerationPanelRecoverySelections(response.ResourceDescriptors)
	return applyGenerationResolvedActionSummaryToNavigationDispatchResponse(response)
}
