package listingkit

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
	for _, builder := range previewPlatformBuilders() {
		if err := builder.build(result, preview, selectedPlatform); err != nil {
			return err
		}
	}
	return nil
}
