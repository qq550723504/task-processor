package listingkit

type previewPlatformBuilder interface {
	platform() string
	build(task *Task, preview *ListingKitPreview, selectedPlatform string) error
}

type previewPlatformBuilderFunc struct {
	name string
	fn   func(task *Task, preview *ListingKitPreview, selectedPlatform string) error
}

func (b previewPlatformBuilderFunc) platform() string {
	return b.name
}

func (b previewPlatformBuilderFunc) build(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	return b.fn(task, preview, selectedPlatform)
}

func previewPlatformBuilders() []previewPlatformBuilder {
	registrations := previewPlatformRegistrations()
	builders := make([]previewPlatformBuilder, 0, len(registrations))
	for _, registration := range registrations {
		builders = append(builders, previewPlatformBuilderFunc{name: registration.name, fn: registration.build})
	}
	return builders
}

func buildPreviewPlatformSections(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	for _, builder := range previewPlatformBuilders() {
		if err := builder.build(task, preview, selectedPlatform); err != nil {
			return err
		}
	}
	return nil
}
