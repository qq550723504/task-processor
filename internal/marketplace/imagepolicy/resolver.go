package imagepolicy

import productimage "task-processor/internal/product/image"

type Resolver struct {
	policies map[PolicyKey]ProductImageProfile
}

func NewResolver(set PolicySet) (*Resolver, error) {
	if !validPolicyVersion(set.Version) || len(set.Policies) == 0 || len(set.Policies) > maxPoliciesPerSet {
		return nil, ErrInvalidPolicySet
	}
	usedBytes := len(set.Version)
	policies := make(map[PolicyKey]ProductImageProfile, len(set.Policies))
	for _, policy := range set.Policies {
		if !validPolicyKey(policy.Key) || !validThresholds(policy.Thresholds) {
			return nil, ErrInvalidPolicySet
		}
		if _, duplicate := policies[policy.Key]; duplicate {
			return nil, ErrInvalidPolicySet
		}
		options, err := productimage.MergeSceneOptions(nil, &policy.SceneDefaults)
		if err != nil || options.SceneCategory != policy.Key.SceneCategory || !addPolicyBytes(&usedBytes, policy) {
			return nil, ErrInvalidPolicySet
		}
		policies[policy.Key] = ProductImageProfile{
			Key:           policy.Key,
			PolicyVersion: set.Version,
			Thresholds:    policy.Thresholds,
			SceneDefaults: cloneSceneOptions(*options),
		}
	}
	return &Resolver{policies: policies}, nil
}

func (r *Resolver) Resolve(input ProfileInput) (ProductImageProfile, error) {
	if r == nil || !validPolicyKey(PolicyKey(input)) {
		return ProductImageProfile{}, ErrInvalidProfileInput
	}
	profile, ok := r.policies[PolicyKey(input)]
	if !ok {
		return ProductImageProfile{}, ErrPolicyNotFound
	}
	profile.SceneDefaults = cloneSceneOptions(profile.SceneDefaults)
	return profile, nil
}

func cloneSceneOptions(options productimage.SceneOptions) productimage.SceneOptions {
	options.StyleReferenceIDs = append([]string(nil), options.StyleReferenceIDs...)
	return options
}
