package productimage

import imagepolicy "task-processor/internal/marketplace/imagepolicy"

// LoadEmbeddedResolver builds the exact resolver shared by command admission
// and worker execution. Keeping this conversion beside the catalog prevents
// the two process compositions from drifting onto different policy key sets.
func LoadEmbeddedResolver() (*imagepolicy.Resolver, error) {
	catalog, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}
	return NewResolver(catalog)
}

func NewResolver(catalog Catalog) (*imagepolicy.Resolver, error) {
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
