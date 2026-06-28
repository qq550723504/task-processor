package listingkit

import listingplatform "task-processor/internal/listing/platform"

type platformSectionBuildFunc[T any] func(result *ListingKitResult, target T, selectedPlatform string) error

type platformSectionRegistration[T any] struct {
	name  string
	build platformSectionBuildFunc[T]
}

type platformSectionBuilder[T any] struct {
	name string
	fn   platformSectionBuildFunc[T]
}

func (b platformSectionBuilder[T]) platform() string {
	return b.name
}

func (b platformSectionBuilder[T]) build(result *ListingKitResult, target T, selectedPlatform string) error {
	return b.fn(result, target, selectedPlatform)
}

func platformSectionBuilders[T any](registrations []platformSectionRegistration[T]) []platformSectionBuilder[T] {
	builders := make([]platformSectionBuilder[T], 0, len(registrations))
	for _, registration := range registrations {
		builders = append(builders, platformSectionBuilder[T]{name: registration.name, fn: registration.build})
	}
	return builders
}

func buildPlatformSections[T any](builders []platformSectionBuilder[T], result *ListingKitResult, target T, selectedPlatform string) error {
	sectionBuilders := make([]listingplatform.Builder, 0, len(builders))
	for _, builder := range builders {
		builder := builder
		sectionBuilders = append(sectionBuilders, listingplatform.Builder{
			Platform: builder.platform(),
			Build: func() error {
				return builder.build(result, target, selectedPlatform)
			},
		})
	}
	return listingplatform.BuildAll(sectionBuilders)
}
