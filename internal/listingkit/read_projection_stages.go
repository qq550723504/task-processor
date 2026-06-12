package listingkit

func calculateListingKitNeedsReview(result *ListingKitResult) bool {
	return result != nil && result.Summary != nil && result.Summary.NeedsReview
}

func assembleListingKitReadProjection(
	needsReview bool,
	overview *listingKitOverviewData,
	attachment *listingKitResultAttachment,
) *listingKitReadProjection {
	return &listingKitReadProjection{
		NeedsReview: needsReview,
		Overview:    overview,
		Attachment:  attachment,
	}
}
