package listingkit

import listingplatform "task-processor/internal/listing/platform"

func previewPlatformRegistrations() []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitPreview] {
	return []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitPreview]{
		{Platform: "amazon", Build: buildAmazonPreviewSection},
		{Platform: "shein", Build: buildSheinPreviewSection},
		{Platform: "temu", Build: buildTemuPreviewSection},
		{Platform: "walmart", Build: buildWalmartPreviewSection},
	}
}
