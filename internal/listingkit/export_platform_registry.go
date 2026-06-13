package listingkit

type exportPlatformRegistration struct {
	name  string
	build func(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error
}

func exportPlatformRegistrations() []exportPlatformRegistration {
	return []exportPlatformRegistration{
		{name: "amazon", build: buildAmazonExportSection},
		{name: "shein", build: buildSheinExportSection},
		{name: "temu", build: buildTemuExportSection},
		{name: "walmart", build: buildWalmartExportSection},
	}
}
