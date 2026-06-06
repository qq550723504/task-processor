package listingkit

import "strings"

func cloneGenerationConditionalState(state *GenerationConditionalState) *GenerationConditionalState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func ApplyGenerationConditionalBaselineToNavigationTarget(target *GenerationReviewNavigationTarget, fallbackIfMatch string) {
	if target == nil {
		return
	}
	baseline := strings.TrimSpace(fallbackIfMatch)
	if target.Conditional != nil && strings.TrimSpace(target.Conditional.DeltaToken) != "" {
		baseline = strings.TrimSpace(target.Conditional.DeltaToken)
	}
	if baseline == "" {
		return
	}
	applyGenerationConditionalBaselineToQuery(target.QueueQuery, baseline)
	applyGenerationConditionalBaselineToQuery(target.SessionQuery, baseline)
	applyGenerationConditionalBaselineToQuery(target.PreviewQuery, baseline)
	if target.ActionTarget != nil {
		applyGenerationConditionalBaselineToQuery(target.ActionTarget.QueueQuery, baseline)
	}
}

func applyGenerationConditionalBaselineToQuery(query *GenerationQueueQuery, baseline string) {
	if query == nil {
		return
	}
	if strings.TrimSpace(query.IfMatch) != "" || strings.TrimSpace(query.DeltaToken) != "" {
		return
	}
	query.IfMatch = strings.TrimSpace(baseline)
}

func applyConditionalBaselineToQuery(query *GenerationQueueQuery, state *GenerationConditionalState) *GenerationQueueQuery {
	if query == nil || state == nil || strings.TrimSpace(state.DeltaToken) == "" {
		return query
	}
	if strings.TrimSpace(query.IfMatch) == "" && strings.TrimSpace(query.DeltaToken) == "" {
		query.IfMatch = state.DeltaToken
	}
	return query
}

func applyConditionalStateToNavigationTarget(target *GenerationReviewNavigationTarget, state *GenerationConditionalState) *GenerationReviewNavigationTarget {
	if target == nil || state == nil {
		return target
	}
	target.Conditional = cloneGenerationConditionalState(state)
	target.QueueQuery = applyConditionalBaselineToQuery(target.QueueQuery, state)
	target.SessionQuery = applyConditionalBaselineToQuery(target.SessionQuery, state)
	target.PreviewQuery = applyConditionalBaselineToQuery(target.PreviewQuery, state)
	return applyIdentityToNavigationTarget(target)
}

func applyConditionalStateToReviewTarget(target *GenerationReviewTarget, state *GenerationConditionalState) *GenerationReviewTarget {
	if target == nil || state == nil {
		return target
	}
	target.NavigationTarget = applyConditionalStateToNavigationTarget(target.NavigationTarget, state)
	target.QueueQuery = applyConditionalBaselineToQuery(target.QueueQuery, state)
	target.SessionQuery = applyConditionalBaselineToQuery(target.SessionQuery, state)
	return target
}

func applyConditionalStateToPreviewViewer(viewer *GenerationReviewPreviewViewer, state *GenerationConditionalState) *GenerationReviewPreviewViewer {
	if viewer == nil || state == nil {
		return viewer
	}
	viewer.NavigationTarget = applyConditionalStateToNavigationTarget(viewer.NavigationTarget, state)
	viewer.PreviewQuery = applyConditionalBaselineToQuery(viewer.PreviewQuery, state)
	return viewer
}

func applyConditionalStateToToolbarAction(action *GenerationReviewToolbarAction, state *GenerationConditionalState) *GenerationReviewToolbarAction {
	if action == nil || state == nil {
		return action
	}
	action.Target = applyConditionalStateToReviewTarget(action.Target, state)
	action.ViewerTarget = applyConditionalStateToPreviewViewer(action.ViewerTarget, state)
	action.NavigationTarget = applyConditionalStateToNavigationTarget(action.NavigationTarget, state)
	action.PreviewQuery = applyConditionalBaselineToQuery(action.PreviewQuery, state)
	if action.ActionTarget != nil {
		action.ActionTarget.NavigationTarget = applyConditionalStateToNavigationTarget(action.ActionTarget.NavigationTarget, state)
	}
	return action
}

func applyConditionalStateToToolbarInput(input *GenerationReviewToolbarInput, state *GenerationConditionalState) *GenerationReviewToolbarInput {
	if input == nil || state == nil {
		return input
	}
	input.PreviewViewer = applyConditionalStateToPreviewViewer(input.PreviewViewer, state)
	for i := range input.SectionActions {
		applyConditionalStateToToolbarAction(&input.SectionActions[i], state)
	}
	for i := range input.PreviewActions {
		applyConditionalStateToToolbarAction(&input.PreviewActions[i], state)
	}
	return input
}

func applyConditionalStateToReviewSection(section *GenerationReviewSection, state *GenerationConditionalState) *GenerationReviewSection {
	if section == nil || state == nil {
		return section
	}
	section.PrimaryActionTarget = applyConditionalStateToReviewTarget(section.PrimaryActionTarget, state)
	section.ReviewTarget = applyConditionalStateToReviewTarget(section.ReviewTarget, state)
	for i := range section.ToolbarActions {
		applyConditionalStateToToolbarAction(&section.ToolbarActions[i], state)
	}
	for i := range section.WorkflowActions {
		applyConditionalStateToToolbarAction(&section.WorkflowActions[i], state)
	}
	return section
}

func applyConditionalStateToReviewSlot(slot *GenerationReviewSlot, state *GenerationConditionalState) *GenerationReviewSlot {
	if slot == nil || state == nil {
		return slot
	}
	slot.ReviewTarget = applyConditionalStateToReviewTarget(slot.ReviewTarget, state)
	return slot
}

func applyConditionalStateToPlatformCard(card *ListingKitPlatformCard, state *GenerationConditionalState) *ListingKitPlatformCard {
	if card == nil || state == nil {
		return card
	}
	card.ReviewTarget = applyConditionalStateToReviewTarget(card.ReviewTarget, state)
	if card.PrimaryActionTarget != nil {
		card.PrimaryActionTarget.NavigationTarget = applyConditionalStateToNavigationTarget(card.PrimaryActionTarget.NavigationTarget, state)
	}
	return card
}

func applyConditionalStateToReviewSession(session *GenerationReviewSession, state *GenerationConditionalState) *GenerationReviewSession {
	if session == nil || state == nil {
		return session
	}
	session.DefaultTarget = applyConditionalStateToReviewTarget(session.DefaultTarget, state)
	session.FocusedTarget = applyConditionalStateToReviewTarget(session.FocusedTarget, state)
	session.FocusedToolbar = applyConditionalStateToToolbarInput(session.FocusedToolbar, state)
	for i := range session.PlatformCards {
		applyConditionalStateToPlatformCard(&session.PlatformCards[i], state)
	}
	for i := range session.SlotNavigation {
		applyConditionalStateToReviewSlot(&session.SlotNavigation[i], state)
	}
	for i := range session.Sections {
		applyConditionalStateToReviewSection(&session.Sections[i], state)
	}
	return session
}

func applyConditionalStateToReviewPatch(patch *GenerationReviewSessionPatch, state *GenerationConditionalState) *GenerationReviewSessionPatch {
	if patch == nil || state == nil {
		return patch
	}
	patch.FocusedTarget = applyConditionalStateToReviewTarget(patch.FocusedTarget, state)
	patch.FocusedToolbar = applyConditionalStateToToolbarInput(patch.FocusedToolbar, state)
	for i := range patch.ChangedSections {
		applyConditionalStateToReviewSection(&patch.ChangedSections[i], state)
	}
	for i := range patch.ChangedSlots {
		applyConditionalStateToReviewSlot(&patch.ChangedSlots[i], state)
	}
	for i := range patch.ChangedPlatformCards {
		applyConditionalStateToPlatformCard(&patch.ChangedPlatformCards[i], state)
	}
	if patch.Focus != nil {
		patch.Focus.FocusedTarget = applyConditionalStateToReviewTarget(patch.Focus.FocusedTarget, state)
		patch.Focus.FocusedToolbar = applyConditionalStateToToolbarInput(patch.Focus.FocusedToolbar, state)
	}
	if patch.Queue != nil {
		for i := range patch.Queue.ChangedSections {
			applyConditionalStateToReviewSection(&patch.Queue.ChangedSections[i], state)
		}
		for i := range patch.Queue.ChangedSlots {
			applyConditionalStateToReviewSlot(&patch.Queue.ChangedSlots[i], state)
		}
	}
	if patch.PlatformCards != nil {
		for i := range patch.PlatformCards.Items {
			applyConditionalStateToPlatformCard(&patch.PlatformCards.Items[i], state)
		}
	}
	return patch
}
