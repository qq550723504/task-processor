package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildGenerationConditionalState(deltaToken string, notModified bool, noChanges bool) *GenerationConditionalState {
	state := listinggeneration.BuildConditionalState(deltaToken, notModified, noChanges)
	if state == nil {
		return nil
	}
	return &GenerationConditionalState{
		DeltaToken:  state.DeltaToken,
		ETag:        state.ETag,
		NotModified: state.NotModified,
		NoChanges:   state.NoChanges,
	}
}

func buildGenerationConditionalETag(deltaToken string) string {
	return listinggeneration.ConditionalETag(deltaToken)
}

func buildGenerationQueueResponseDescriptors(page *GenerationQueuePage) []GenerationPanelResourceDescriptor {
	if page == nil || page.NotModified {
		return nil
	}
	out := make([]GenerationPanelResourceDescriptor, 0, len(page.Items))
	for _, item := range page.Items {
		target := &GenerationReviewNavigationTarget{
			DispatchKind: "queue",
			QueueQuery: &GenerationQueueQuery{
				Platform:          item.Platform,
				Slot:              item.Slot,
				PreviewCapability: firstQueuePreviewCapability(item),
			},
		}
		target = applyIdentityToNavigationTarget(target)
		if target.Descriptor == nil {
			continue
		}
		descriptor := GenerationPanelResourceDescriptor{
			Role:          "queue_item",
			Platform:      item.Platform,
			Slot:          item.Slot,
			Capability:    firstQueuePreviewCapability(item),
			RecoveryScope: "queue_item",
			RecoveryHint:  item.RetryHint,
			Retryable:     item.Retryable,
			Descriptor:    cloneGenerationNavigationDescriptor(target.Descriptor),
		}
		applyGenerationPanelResourceRecovery(&descriptor)
		out = append(out, descriptor)
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationSessionResponseDescriptors(response *GenerationReviewSessionResponse) []GenerationPanelResourceDescriptor {
	if response == nil || response.NotModified {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	if response.Session != nil {
		update := &GenerationReviewPanelUpdate{
			FocusedTarget:  response.Session.FocusedTarget,
			FocusedToolbar: response.Session.FocusedToolbar,
		}
		out = append(out, buildGenerationPanelFocusedDescriptors(update)...)
	}
	if response.Patch != nil {
		update := &GenerationReviewPanelUpdate{ReviewPatch: response.Patch}
		out = append(out, buildGenerationPanelChangedDescriptors(update)...)
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationPreviewResponseDescriptors(response *GenerationReviewPreviewResponse) []GenerationPanelResourceDescriptor {
	if response == nil || response.NotModified {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	if item := buildGenerationPanelViewerDescriptor("preview_viewer", response.Viewer); item != nil {
		out = append(out, *item)
	}
	if item := buildGenerationPanelTargetDescriptor("preview_session", response.ReviewTarget); item != nil {
		out = append(out, *item)
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationActionResponseDescriptors(result *GenerationActionExecutionResult) []GenerationPanelResourceDescriptor {
	if result == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	if navigation := actionResponseNavigationTarget(result.ResolvedTarget); navigation != nil && navigation.Descriptor != nil {
		out = append(out, GenerationPanelResourceDescriptor{
			Role:       "action_target",
			Descriptor: cloneGenerationNavigationDescriptor(navigation.Descriptor),
		})
	}
	out = append(out, buildGenerationQueueResponseDescriptors(result.Queue)...)
	out = append(out, buildGenerationSessionResponseDescriptors(&GenerationReviewSessionResponse{
		Session: result.ReviewSession,
		Patch:   result.ReviewPatch,
	})...)
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationDispatchResponseDescriptors(response *GenerationReviewNavigationDispatchResponse) []GenerationPanelResourceDescriptor {
	if response == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	out = append(out, buildGenerationQueueResponseDescriptors(response.Queue)...)
	out = append(out, buildGenerationSessionResponseDescriptors(response.ReviewSession)...)
	out = append(out, buildGenerationPreviewResponseDescriptors(response.ReviewPreview)...)
	out = append(out, buildGenerationActionResponseDescriptors(response.Action)...)
	if response.PanelUpdate != nil {
		out = append(out, response.PanelUpdate.FocusedDescriptors...)
		out = append(out, response.PanelUpdate.ChangedDescriptors...)
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func firstQueuePreviewCapability(item GenerationWorkQueueItem) string {
	if len(item.PreviewCapabilities) == 0 {
		return ""
	}
	return item.PreviewCapabilities[0]
}

func actionResponseNavigationTarget(target *AssetGenerationActionTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	if target.NavigationTarget != nil {
		return target.NavigationTarget
	}
	return buildGenerationReviewActionNavigationTarget(target)
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
