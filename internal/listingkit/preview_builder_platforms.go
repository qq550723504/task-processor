package listingkit

func buildPreviewPlatformSections(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	for _, builder := range previewPlatformBuilders() {
		if err := builder.build(task, preview, selectedPlatform); err != nil {
			return err
		}
	}
	return nil
}
