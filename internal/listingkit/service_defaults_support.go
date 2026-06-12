package listingkit

func runServiceConfigInitializers(config *ServiceConfig, initializers ...func(*ServiceConfig)) {
	if config == nil {
		return
	}
	for _, initializer := range initializers {
		if initializer == nil {
			continue
		}
		initializer(config)
	}
}
