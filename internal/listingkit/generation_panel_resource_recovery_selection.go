package listingkit

import "sort"

func buildGenerationPanelRecoverySelections(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {
	primary, recommended := selectGenerationPanelRecoveryDescriptors(items)
	return primary, recommended
}

func selectGenerationPanelRecoveryDescriptors(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {
	if len(items) == 0 {
		return nil, nil
	}
	recoverable := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		if !isGenerationPanelResourceRecoverable(item) {
			continue
		}
		recoverable = append(recoverable, item)
	}
	if len(recoverable) == 0 {
		return nil, nil
	}
	sort.SliceStable(recoverable, func(i, j int) bool {
		li := generationPanelRecoveryPriority(recoverable[i])
		lj := generationPanelRecoveryPriority(recoverable[j])
		if li != lj {
			return li < lj
		}
		if recoverable[i].Role != recoverable[j].Role {
			return recoverable[i].Role < recoverable[j].Role
		}
		return generationPanelResourceCacheKey(recoverable[i]) < generationPanelResourceCacheKey(recoverable[j])
	})
	primary := recoverable[0]
	return &primary, recoverable
}

func isGenerationPanelResourceRecoverable(item GenerationPanelResourceDescriptor) bool {
	if item.RecoveryHint == "" {
		return false
	}
	return item.RecoveryTarget != nil || item.Retryable
}

func generationPanelRecoveryPriority(item GenerationPanelResourceDescriptor) int {
	return generationRecoveryProfileForHint(item.RecoveryHint).Priority
}

func generationPanelResourceCacheKey(item GenerationPanelResourceDescriptor) string {
	if item.Descriptor == nil {
		return ""
	}
	return item.Descriptor.CacheKey
}
