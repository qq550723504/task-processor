package listingkit

func cloneGenerationNavigationFollowUpReadSlice(items []GenerationNavigationFollowUpRead) []GenerationNavigationFollowUpRead {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]GenerationNavigationFollowUpRead, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneGenerationNavigationFollowUpRead(item))
	}
	return cloned
}
