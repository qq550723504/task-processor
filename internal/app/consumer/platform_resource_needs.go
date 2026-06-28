package consumer

import "task-processor/internal/core/config"

type PlatformResourceNeedsResolver struct {
	cfg     *config.Config
	catalog platformModuleCatalog
}

func NewPlatformResourceNeedsResolver(cfg *config.Config, catalog platformModuleCatalog) PlatformResourceNeedsResolver {
	return PlatformResourceNeedsResolver{
		cfg:     cfg,
		catalog: catalog,
	}
}

func (r PlatformResourceNeedsResolver) Resolve(platforms ...string) (SharedResourceNeeds, error) {
	modules, err := r.catalog.resolveMany(platforms...)
	if err != nil {
		return SharedResourceNeeds{}, err
	}
	return SharedResourceNeeds{
		NeedAmazonCrawler: r.anyModuleNeedsAmazon(modules),
	}, nil
}

func (r PlatformResourceNeedsResolver) anyModuleNeedsAmazon(modules []PlatformModule) bool {
	for _, module := range modules {
		name := module.Name()
		if module.NeedsAmazon(r.cfg) || PlatformUsesLocalFetcher(r.cfg, name) {
			return true
		}
	}
	return false
}
