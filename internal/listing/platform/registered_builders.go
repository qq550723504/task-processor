package platform

type SectionBuildFunc[C, T any] func(context C, target T, selectedPlatform string) error

type SectionRegistration[C, T any] struct {
	Platform string
	Build    SectionBuildFunc[C, T]
}

type RegisteredSectionBuilder[C, T any] struct {
	platform string
	build    SectionBuildFunc[C, T]
}

func (b RegisteredSectionBuilder[C, T]) Platform() string {
	return b.platform
}

func (b RegisteredSectionBuilder[C, T]) Build(context C, target T, selectedPlatform string) error {
	return b.build(context, target, selectedPlatform)
}

func SectionBuilders[C, T any](registrations []SectionRegistration[C, T]) []RegisteredSectionBuilder[C, T] {
	builders := make([]RegisteredSectionBuilder[C, T], 0, len(registrations))
	for _, registration := range registrations {
		builders = append(builders, RegisteredSectionBuilder[C, T]{
			platform: registration.Platform,
			build:    registration.Build,
		})
	}
	return builders
}

func SupportedSectionRegistrations[C, T any](builds map[string]SectionBuildFunc[C, T]) []SectionRegistration[C, T] {
	registrations := make([]SectionRegistration[C, T], 0, len(builds))
	for _, platform := range supportedPlatforms {
		build, ok := builds[platform]
		if !ok {
			continue
		}
		registrations = append(registrations, SectionRegistration[C, T]{
			Platform: platform,
			Build:    build,
		})
	}
	return registrations
}

func BuildRegisteredSections[C, T any](builders []RegisteredSectionBuilder[C, T], context C, target T, selectedPlatform string) error {
	sectionBuilders := make([]Builder, 0, len(builders))
	for _, builder := range builders {
		builder := builder
		sectionBuilders = append(sectionBuilders, Builder{
			Platform: builder.Platform(),
			Build: func() error {
				return builder.Build(context, target, selectedPlatform)
			},
		})
	}
	return BuildAll(sectionBuilders)
}
