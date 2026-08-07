package listingkit

func cloneSourceReference(source *SourceReference) *SourceReference {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}
