package listingkit

func buildListingKitResultAttachment(result *ListingKitResult, selectedPlatform string) *listingKitResultAttachment {
	attachment := initializeListingKitResultAttachment(result)
	attachment = backfillListingKitResultAttachment(result, attachment)
	return selectListingKitResultAttachmentPlatform(attachment, selectedPlatform)
}
