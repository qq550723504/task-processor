package listingkit

func cloneGenerationNavigationFollowUpRead(item GenerationNavigationFollowUpRead) GenerationNavigationFollowUpRead {
	cloned := item
	applyGenerationNavigationFollowUpReadCloneShape(item, &cloned)
	return cloned
}
