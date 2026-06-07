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

func buildGenerationPanelFocusedDescriptors(update *GenerationReviewPanelUpdate) []GenerationPanelResourceDescriptor {
	if update == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	if item := buildGenerationPanelTargetDescriptor("focused_session", update.FocusedTarget); item != nil {
		applyGenerationPanelFocusedSourceMetadata(item, update)
		out = append(out, *item)
	}
	if update.FocusedToolbar != nil && update.FocusedToolbar.PreviewViewer != nil {
		if item := buildGenerationPanelViewerDescriptor("focused_preview", update.FocusedToolbar.PreviewViewer); item != nil {
			applyGenerationPanelFocusedSourceMetadata(item, update)
			out = append(out, *item)
		}
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationPanelChangedDescriptors(update *GenerationReviewPanelUpdate) []GenerationPanelResourceDescriptor {
	if update == nil || update.ReviewPatch == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	for _, section := range update.ReviewPatch.ChangedSections {
		if item := buildGenerationPanelTargetDescriptor("changed_section", section.ReviewTarget); item != nil {
			item.SectionKey = section.SectionKey
			out = append(out, *item)
		}
	}
	for _, slot := range update.ReviewPatch.ChangedSlots {
		if item := buildGenerationPanelTargetDescriptor("changed_slot", slot.ReviewTarget); item != nil {
			out = append(out, *item)
		}
	}
	for _, card := range update.ReviewPatch.ChangedPlatformCards {
		if item := buildGenerationPanelTargetDescriptor("changed_platform_card", card.ReviewTarget); item != nil {
			out = append(out, *item)
		}
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationPanelTargetDescriptor(role string, target *GenerationReviewTarget) *GenerationPanelResourceDescriptor {
	if target == nil || target.NavigationTarget == nil || target.NavigationTarget.Descriptor == nil {
		return nil
	}
	return &GenerationPanelResourceDescriptor{
		Role:       role,
		Platform:   target.Platform,
		Slot:       target.Slot,
		Capability: target.Capability,
		SectionKey: target.SectionKey,
		Descriptor: cloneGenerationNavigationDescriptor(target.NavigationTarget.Descriptor),
	}
}

func buildGenerationPanelViewerDescriptor(role string, viewer *GenerationReviewPreviewViewer) *GenerationPanelResourceDescriptor {
	if viewer == nil || viewer.NavigationTarget == nil || viewer.NavigationTarget.Descriptor == nil {
		return nil
	}
	return &GenerationPanelResourceDescriptor{
		Role:       role,
		Platform:   viewer.Platform,
		Slot:       viewer.Slot,
		Descriptor: cloneGenerationNavigationDescriptor(viewer.NavigationTarget.Descriptor),
	}
}

func applyGenerationPanelFocusedSourceMetadata(item *GenerationPanelResourceDescriptor, update *GenerationReviewPanelUpdate) {
	if item == nil || update == nil {
		return
	}
	item.SourceKind = update.FocusedSourceKind
	item.SourceStep = update.FocusedSourceStep
	item.ViaFallback = update.FocusedViaFallback
	item.FallbackReason = update.FocusedFallbackReason
	if update.FocusedViaFallback {
		item.RecoveryScope = "focused_resource"
		item.RecoveryHint = "review_fallback"
		item.Retryable = false
	}
	applyGenerationPanelResourceRecovery(item)
}

func uniqueGenerationPanelResourceDescriptors(items []GenerationPanelResourceDescriptor) []GenerationPanelResourceDescriptor {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		if item.Descriptor == nil || item.Descriptor.CacheKey == "" {
			continue
		}
		key := item.Role + "|" + item.Descriptor.CacheKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
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
	update.PrimaryRecoveryDescriptor, update.RecommendedRecoveryDescriptors = selectGenerationPanelRecoveryDescriptors(append(append([]GenerationPanelResourceDescriptor{}, update.FocusedDescriptors...), update.ChangedDescriptors...))
	return update
}

func applyGenerationConditionalStateToNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	response.Conditional = buildGenerationConditionalState(response.DeltaToken, response.NotModified, false)
	response.ResourceDescriptors = buildGenerationDispatchResponseDescriptors(response)
	response.PrimaryRecoveryDescriptor, response.RecommendedRecoveryDescriptors = selectGenerationPanelRecoveryDescriptors(response.ResourceDescriptors)
	return applyGenerationResolvedActionSummaryToNavigationDispatchResponse(response)
}

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
