package listingkit

type reviewablePlatformPreviewPayloadBase struct {
	platformVisualPreviewPayloadBase
	headline    string
	needsReview bool
	reviewNotes []string
}

func buildReviewablePlatformPreviewPayloadBase(headline string, base reviewablePlatformPreviewBase) reviewablePlatformPreviewPayloadBase {
	return reviewablePlatformPreviewPayloadBase{
		platformVisualPreviewPayloadBase: base.platformVisualPresentationBase,
		headline:                         headline,
		needsReview:                      base.needsReview,
		reviewNotes:                      base.reviewNotes,
	}
}
