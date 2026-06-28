package listingkit

func exportPlatformRegistrations() []platformSectionRegistration[*ListingKitExport] {
	return []platformSectionRegistration[*ListingKitExport]{
		{name: "amazon", build: buildAmazonExportSection},
		{name: "shein", build: buildSheinExportSection},
		{name: "temu", build: buildTemuExportSection},
		{name: "walmart", build: buildWalmartExportSection},
	}
}
