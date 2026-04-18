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
