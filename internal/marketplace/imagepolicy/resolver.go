package imagepolicy

import (
	"strings"

	productimage "task-processor/internal/product/image"
)

type Resolver struct {
	policies map[PolicyKey]ProductImageProfile
}

func NewResolver(set PolicySet) (*Resolver, error) {
	if err := validatePolicySet(set); err != nil {
		return nil, err
	}
	return buildResolver(set)
}

func validatePolicySet(set PolicySet) error {
	if !validPolicyVersion(set.Version) || len(set.Policies) == 0 || len(set.Policies) > maxPoliciesPerSet {
		return ErrInvalidPolicySet
	}
	usedBytes := len(set.Version)
	seen := make(map[PolicyKey]struct{}, len(set.Policies))
	for _, policy := range set.Policies {
		if !validPolicyKey(policy.Key) || !validThresholds(policy.Thresholds) {
			return ErrInvalidPolicySet
		}
		if _, duplicate := seen[policy.Key]; duplicate {
			return ErrInvalidPolicySet
		}
		seen[policy.Key] = struct{}{}
		if !addPolicyBytes(&usedBytes, policy) {
			return ErrInvalidPolicySet
		}
	}
	for _, policy := range set.Policies {
		options, err := productimage.MergeSceneOptions(nil, &policy.SceneDefaults)
		if err != nil || options.SceneCategory != policy.Key.SceneCategory {
			return ErrInvalidPolicySet
		}
	}
	return nil
}

func buildResolver(set PolicySet) (*Resolver, error) {
	policies := make(map[PolicyKey]ProductImageProfile, len(set.Policies))
	version := strings.Clone(set.Version)
	for _, policy := range set.Policies {
		options, err := productimage.MergeSceneOptions(nil, &policy.SceneDefaults)
		if err != nil {
			return nil, ErrInvalidPolicySet
		}
		key := ownPolicyKey(policy.Key)
		policies[key] = ProductImageProfile{
			Key:           key,
			PolicyVersion: version,
			Thresholds:    policy.Thresholds,
			SceneDefaults: ownSceneOptions(*options),
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

func ownPolicyKey(key PolicyKey) PolicyKey {
	return PolicyKey{
		Marketplace:   strings.Clone(key.Marketplace),
		Country:       strings.Clone(key.Country),
		Family:        strings.Clone(key.Family),
		SceneCategory: strings.Clone(key.SceneCategory),
	}
}

func ownSceneOptions(options productimage.SceneOptions) productimage.SceneOptions {
	owned := productimage.SceneOptions{
		SceneCategory:   strings.Clone(options.SceneCategory),
		SceneStyle:      strings.Clone(options.SceneStyle),
		BackgroundTone:  strings.Clone(options.BackgroundTone),
		Composition:     strings.Clone(options.Composition),
		PropsLevel:      strings.Clone(options.PropsLevel),
		AudienceHint:    strings.Clone(options.AudienceHint),
		CustomSceneHint: strings.Clone(options.CustomSceneHint),
		SlotRole:        strings.Clone(options.SlotRole),
		SlotBrief:       strings.Clone(options.SlotBrief),
	}
	if len(options.StyleReferenceIDs) > 0 {
		owned.StyleReferenceIDs = make([]string, len(options.StyleReferenceIDs))
		for index, referenceID := range options.StyleReferenceIDs {
			owned.StyleReferenceIDs[index] = strings.Clone(referenceID)
		}
	}
	return owned
}
