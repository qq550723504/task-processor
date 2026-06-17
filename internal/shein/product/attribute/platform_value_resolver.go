package attribute

type platformValueResolver interface {
	Resolve(attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string)
}

type platformValueResolverRegistry struct {
	resolvers map[platformValueDomain]platformValueResolver
}

func newPlatformValueResolverRegistry() *platformValueResolverRegistry {
	return &platformValueResolverRegistry{
		resolvers: map[platformValueDomain]platformValueResolver{
			platformValueDomainShoeMetricSize:     shoeMetricSizeResolver{},
			platformValueDomainApparelAlphaSize:   apparelAlphaSizeResolver{},
			platformValueDomainApparelNumericSize: apparelNumericSizeResolver{},
		},
	}
}

func (r *platformValueResolverRegistry) Resolve(domain platformValueDomain, attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if r == nil {
		return 0, ""
	}
	resolver, ok := r.resolvers[domain]
	if !ok || resolver == nil {
		return 0, ""
	}
	return resolver.Resolve(attrID, rawValue, runtime, platformValues, matcher)
}

type shoeMetricSizeResolver struct{}

func (shoeMetricSizeResolver) Resolve(attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	return resolveShoeSizePlatformID(attrID, rawValue, runtime, platformValues, matcher)
}

type apparelAlphaSizeResolver struct{}

func (apparelAlphaSizeResolver) Resolve(attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if matcher == nil {
		return 0, ""
	}

	candidates := make([]string, 0, 3)
	if normalized, ok := normalizeAlphaSizeLabel(rawValue); ok {
		candidates = append(candidates, normalized)
	}
	candidates = append(candidates, rawValue)

	if runtime != nil && runtime.AmazonProduct != nil && runtime.AmazonProduct.SizeChart != nil {
		if normalized, ok := findNormalizedAlphaSizeFromChart(runtime.AmazonProduct.SizeChart, rawValue); ok {
			candidates = append(candidates, normalized)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if platformID := matcher.FindMatchingPlatformValue(candidate, platformValues); platformID > 0 {
			return platformID, candidate
		}
	}

	return 0, ""
}

type apparelNumericSizeResolver struct{}

func (apparelNumericSizeResolver) Resolve(attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if matcher == nil {
		return 0, ""
	}

	for _, candidate := range buildApparelNumericSizeCandidates(rawValue, runtime) {
		if platformID := matcher.FindMatchingPlatformValue(candidate, platformValues); platformID > 0 {
			return platformID, candidate
		}
	}

	return 0, ""
}
