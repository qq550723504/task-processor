package listingkit

import listingplatform "task-processor/internal/listing/platform"

func exportPlatformRegistrations() []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitExport] {
	return []listingplatform.SectionRegistration[*ListingKitResult, *ListingKitExport]{
		{Platform: "amazon", Build: buildAmazonExportSection},
		{Platform: "shein", Build: buildSheinExportSection},
		{Platform: "temu", Build: buildTemuExportSection},
		{Platform: "walmart", Build: buildWalmartExportSection},
	}
}
