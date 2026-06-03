package listingkit

func applyGenerationNavigationFollowUpReadCloneShape(item GenerationNavigationFollowUpRead, cloned *GenerationNavigationFollowUpRead) {
	if cloned == nil {
		return
	}
	cloned.Query = cloneGenerationQueueQuery(item.Query)
}
