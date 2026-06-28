package consumer

import "task-processor/internal/core/config"

type platformResourceNeedsResolver struct {
	cfg     *config.Config
	catalog platformModuleCatalog
}

func newPlatformResourceNeedsResolver(cfg *config.Config, catalog platformModuleCatalog) platformResourceNeedsResolver {
	return platformResourceNeedsResolver{
		cfg:     cfg,
		catalog: catalog,
	}
}

func (r platformResourceNeedsResolver) resolve(platforms ...string) (SharedResourceNeeds, error) {
	modules, err := r.catalog.resolveMany(platforms...)
	if err != nil {
		return SharedResourceNeeds{}, err
	}
	return SharedResourceNeeds{
		NeedAmazonCrawler: r.anyModuleNeedsAmazon(modules),
	}, nil
}

func (r platformResourceNeedsResolver) anyModuleNeedsAmazon(modules []PlatformModule) bool {
	for _, module := range modules {
		name := module.Name()
		if module.NeedsAmazon(r.cfg) || PlatformUsesLocalFetcher(r.cfg, name) {
			return true
		}
	}
	return false
}
