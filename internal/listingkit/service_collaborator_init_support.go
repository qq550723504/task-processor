package listingkit

func (s *service) runCollaboratorInitializers(initializers ...func()) {
	if s == nil {
		return
	}
	for _, initializer := range initializers {
		if initializer == nil {
			continue
		}
		initializer()
	}
}
