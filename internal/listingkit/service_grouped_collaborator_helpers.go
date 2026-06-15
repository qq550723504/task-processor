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
