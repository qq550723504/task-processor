package imageagentpolicy

import (
	policycatalog "task-processor/internal/integration/policy/productimage"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
)

// LoadEmbeddedResolver builds the exact resolver shared by command admission
// and worker execution. The app composition layer owns this infrastructure to
// domain mapping so the policy catalog remains independent of business rules.
func LoadEmbeddedResolver() (*imagepolicy.Resolver, error) {
	catalog, err := policycatalog.LoadEmbedded()
	if err != nil {
		return nil, err
	}
	return NewResolver(catalog)
}

func NewResolver(catalog policycatalog.Catalog) (*imagepolicy.Resolver, error) {
	set := imagepolicy.PolicySet{Version: catalog.Version, Policies: make([]imagepolicy.Policy, len(catalog.Policies))}
	for index, policy := range catalog.Policies {
		set.Policies[index] = imagepolicy.Policy{
			Key: imagepolicy.PolicyKey{
				Marketplace: policy.Key.Marketplace, Country: policy.Key.Country,
				Family: policy.Key.Family, SceneCategory: policy.Key.SceneCategory,
			},
			Thresholds: imagepolicy.Thresholds{
				MainReview: policy.Thresholds.MainReview, WhiteBackgroundReview: policy.Thresholds.WhiteBackgroundReview,
				WhiteCanvasPenalty: policy.Thresholds.WhiteCanvasPenalty,
			},
			SceneDefaults: policy.SceneDefaults,
		}
	}
	return imagepolicy.NewResolver(set)
}
