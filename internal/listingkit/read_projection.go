package listingkit

type listingKitReadProjection struct {
	NeedsReview bool
	Overview    *listingKitOverviewData
	Attachment  *listingKitResultAttachment
}

func buildListingKitReadProjection(result *ListingKitResult, selectedPlatform string) *listingKitReadProjection {
	if result == nil {
		return nil
	}

	return &listingKitReadProjection{
		NeedsReview: result.Summary != nil && result.Summary.NeedsReview,
		Overview:    buildListingKitOverviewData(result, selectedPlatform),
		Attachment:  buildListingKitResultAttachment(result, selectedPlatform),
	}
}
