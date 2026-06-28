package listingkit

import previewdomain "task-processor/internal/listing/preview"

type exportPlatformBuilder interface {
	platform() string
	build(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error
}

type exportPlatformBuilderFunc struct {
	name string
	fn   func(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error
}

func (b exportPlatformBuilderFunc) platform() string {
	return b.name
}

func (b exportPlatformBuilderFunc) build(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return b.fn(result, export, selectedPlatform)
}

func exportPlatformBuilders() []exportPlatformBuilder {
	registrations := exportPlatformRegistrations()
	builders := make([]exportPlatformBuilder, 0, len(registrations))
	for _, registration := range registrations {
		builders = append(builders, exportPlatformBuilderFunc{name: registration.name, fn: registration.build})
	}
	return builders
}

func buildExportPlatformSections(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	builders := exportPlatformBuilders()
	sectionBuilders := make([]previewdomain.PlatformSectionBuilder, 0, len(builders))
	for _, builder := range builders {
		builder := builder
		sectionBuilders = append(sectionBuilders, previewdomain.PlatformSectionBuilder{
			Platform: builder.platform(),
			Build: func() error {
				return builder.build(result, export, selectedPlatform)
			},
		})
	}
	return previewdomain.BuildPlatformSections(sectionBuilders)
}

func buildAmazonExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "amazon", result != nil && result.Amazon != nil, func() {
		export.Amazon = buildAmazonExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildSheinExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "shein", result != nil && result.Shein != nil, func() {
		export.Shein = buildSheinExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildTemuExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "temu", result != nil && result.Temu != nil, func() {
		export.Temu = buildTemuExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildWalmartExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "walmart", result != nil && result.Walmart != nil, func() {
		export.Walmart = buildWalmartExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}
