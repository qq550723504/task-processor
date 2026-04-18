package listingkit

func buildGenerationRecoverySummaryFromQueue(queue *GenerationWorkQueue) *GenerationRecoverySummary {
	if queue == nil {
		return nil
	}
	descriptors := buildGenerationQueueResponseDescriptors(&GenerationQueuePage{Items: queue.Items})
	return buildGenerationRecoverySummaryFromDescriptors(descriptors)
}

func buildGenerationRecoverySummaryFromDescriptors(items []GenerationPanelResourceDescriptor) *GenerationRecoverySummary {
	primary, recommended := buildGenerationPanelRecoverySelections(items)
	if primary == nil {
		return nil
	}
	profile := generationRecoveryProfileForHint(primary.RecoveryHint)
	return &GenerationRecoverySummary{
		Title:                  profile.Title,
		Summary:                profile.Summary,
		Severity:               primary.RecoverySeverity,
		Urgency:                primary.RecoveryUrgency,
		CTAKind:                primary.RecoveryCTAKind,
		ActionKey:              primary.RecoveryActionKey,
		RecommendedCount:       len(recommended),
		PrimaryDescriptor:      cloneGenerationPanelResourceDescriptor(primary),
		RecommendedDescriptors: cloneGenerationPanelResourceDescriptors(recommended),
	}
}

func cloneGenerationRecoverySummary(summary *GenerationRecoverySummary) *GenerationRecoverySummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.PrimaryDescriptor = cloneGenerationPanelResourceDescriptor(summary.PrimaryDescriptor)
	cloned.RecommendedDescriptors = cloneGenerationPanelResourceDescriptors(summary.RecommendedDescriptors)
	return &cloned
}

func cloneGenerationPanelResourceDescriptor(item *GenerationPanelResourceDescriptor) *GenerationPanelResourceDescriptor {
	if item == nil {
		return nil
	}
	cloned := *item
	cloned.Descriptor = cloneGenerationNavigationDescriptor(item.Descriptor)
	cloned.RecoveryTarget = cloneGenerationReviewNavigationTarget(item.RecoveryTarget)
	cloned.RecoveryDispatchPlan = cloneGenerationNavigationDispatchPlan(item.RecoveryDispatchPlan)
	return &cloned
}

func cloneGenerationPanelResourceDescriptors(items []GenerationPanelResourceDescriptor) []GenerationPanelResourceDescriptor {
	if len(items) == 0 {
		return nil
	}
	out := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		cloned := cloneGenerationPanelResourceDescriptor(&item)
		if cloned == nil {
			continue
		}
		out = append(out, *cloned)
	}
	return out
}
