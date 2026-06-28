package listingkit

import listingplatform "task-processor/internal/listing/platform"

func exportPlatformRegistrations() []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitExport] {
	return listingplatform.SupportedSectionRegistrations(map[string]listingplatform.SectionBuildFunc[*ListingKitResult, *ListingKitExport]{
		"amazon":  buildAmazonExportSection,
		"shein":   buildSheinExportSection,
		"temu":    buildTemuExportSection,
		"walmart": buildWalmartExportSection,
	})
}
