package listingkit

func cloneGenerationNavigationFollowUpRead(item GenerationNavigationFollowUpRead) GenerationNavigationFollowUpRead {
	return GenerationNavigationFollowUpRead{
		Kind:         item.Kind,
		ResponseMode: item.ResponseMode,
		Query:        cloneGenerationQueueQuery(item.Query),
	}
}
