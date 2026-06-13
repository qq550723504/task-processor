package listingkit

type previewPlatformRegistration struct {
	name  string
	build func(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error
}

func previewPlatformRegistrations() []previewPlatformRegistration {
	return []previewPlatformRegistration{
		{name: "amazon", build: buildAmazonPreviewSection},
		{name: "shein", build: buildSheinPreviewSection},
		{name: "temu", build: buildTemuPreviewSection},
		{name: "walmart", build: buildWalmartPreviewSection},
	}
}
