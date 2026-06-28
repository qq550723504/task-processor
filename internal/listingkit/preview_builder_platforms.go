package listingkit

import "task-processor/internal/listing/platformsection"

type previewPlatformBuilder interface {
	platform() string
	build(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error
}

type previewPlatformBuilderFunc struct {
	name string
	fn   func(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error
}

func (b previewPlatformBuilderFunc) platform() string {
	return b.name
}

func (b previewPlatformBuilderFunc) build(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	return b.fn(result, preview, selectedPlatform)
}

func previewPlatformBuilders() []previewPlatformBuilder {
	registrations := previewPlatformRegistrations()
	builders := make([]previewPlatformBuilder, 0, len(registrations))
	for _, registration := range registrations {
		builders = append(builders, previewPlatformBuilderFunc{name: registration.name, fn: registration.build})
	}
	return builders
}

func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	builders := previewPlatformBuilders()
	sectionBuilders := make([]platformsection.Builder, 0, len(builders))
	for _, builder := range builders {
		builder := builder
		sectionBuilders = append(sectionBuilders, platformsection.Builder{
			Platform: builder.platform(),
			Build: func() error {
				return builder.build(result, preview, selectedPlatform)
			},
		})
	}
	return platformsection.BuildAll(sectionBuilders)
}
