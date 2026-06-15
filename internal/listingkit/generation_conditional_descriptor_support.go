package listingkit

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
