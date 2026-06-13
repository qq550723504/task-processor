package listingkit

func newServiceBase(config *ServiceConfig, defaultSettings SheinSettings) *service {
	if config == nil {
		return &service{sheinSettings: defaultSettings}
	}
	return &service{
		repo:          config.Core.Repository,
		sheinSettings: defaultSettings,
	}
}
