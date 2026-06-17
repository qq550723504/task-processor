package listingkit

func attachListingKitPreviewResult(preview *ListingKitPreview, task *Task, selectedPlatform string) {
	if preview == nil {
		return
	}
	projection := buildListingKitPreviewProjection(task, selectedPlatform)
	applyListingKitPreviewProjection(preview, projection)
}
