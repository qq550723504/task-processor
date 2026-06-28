package listingkit

import listingplatform "task-processor/internal/listing/platform"

func previewPlatformRegistrations() []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitPreview] {
	return listingplatform.SupportedSectionRegistrations(map[string]listingplatform.SectionBuildFunc[*ListingKitResult, *ListingKitPreview]{
		"amazon":  buildAmazonPreviewSection,
		"shein":   buildSheinPreviewSection,
		"temu":    buildTemuPreviewSection,
		"walmart": buildWalmartPreviewSection,
	})
}
