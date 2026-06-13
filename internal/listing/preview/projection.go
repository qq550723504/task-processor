package preview

// ProjectionInput captures the platform-neutral preview shell data that can be
// assembled without depending on legacy ListingKit payload types.
type ProjectionInput struct {
	Shell       ShellInput
	NeedsReview bool
	Overview    *HeaderInput
}

func BuildProjection(input ProjectionInput) *Preview {
	preview := BuildShell(input.Shell)
	if preview == nil {
		return nil
	}
	preview.NeedsReview = input.NeedsReview
	if input.Overview != nil {
		preview.Overview = BuildHeader(*input.Overview)
	}
	return preview
}
