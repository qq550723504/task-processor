package listingkit

func previewPlatformRegistrations() []platformSectionRegistration[*ListingKitPreview] {
	return []platformSectionRegistration[*ListingKitPreview]{
		{name: "amazon", build: buildAmazonPreviewSection},
		{name: "shein", build: buildSheinPreviewSection},
		{name: "temu", build: buildTemuPreviewSection},
		{name: "walmart", build: buildWalmartPreviewSection},
	}
}
