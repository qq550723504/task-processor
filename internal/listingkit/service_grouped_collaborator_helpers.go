package listingkit

func groupedCollaboratorOrBuild[T comparable](group *T, build func() T) T {
	var zero T
	if group == nil {
		return zero
	}
	if *group != zero {
		return *group
	}
	if build == nil {
		return zero
	}
	service := build()
	*group = service
	return service
}

func syncGroupedDependency[T comparable](group *T, legacy *T) T {
	var zero T
	if group == nil || legacy == nil {
		return zero
	}
	if *group != zero {
		*legacy = *group
		return *group
	}
	*group = *legacy
	return *legacy
}

func syncGroupedOptionalDependency[T comparable](group *T, groupEnabled *bool, legacy *T, legacyEnabled *bool) (T, bool) {
	var zero T
	if group == nil || groupEnabled == nil || legacy == nil || legacyEnabled == nil {
		return zero, false
	}
	if *group != zero || *groupEnabled {
		*legacy = *group
		*legacyEnabled = *groupEnabled
		return *group, *groupEnabled
	}
	*group = *legacy
	*groupEnabled = *legacyEnabled
	return *legacy, *legacyEnabled
}
