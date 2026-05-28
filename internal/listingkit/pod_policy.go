package listingkit

type podExecutionPolicy struct {
	Provider       string
	DependencyMode string
	DecisionSource string
}

func determinePODExecutionPolicy(req *GenerateRequest) podExecutionPolicy {
	policy := podExecutionPolicy{
		DecisionSource: "system_rule",
	}
	if shouldUseOptionalPODFallback(req) {
		policy.Provider = podProviderSDS
		policy.DependencyMode = podDependencyModeOptional
		return policy
	}
	if shouldUsePODPlatform(req) {
		policy.Provider = podProviderSDS
		policy.DependencyMode = podDependencyModeRequired
		return policy
	}
	policy.DependencyMode = podDependencyModeDisabled
	return policy
}

func shouldUseOptionalPODFallback(req *GenerateRequest) bool {
	if req == nil {
		return false
	}
	if !shouldRunRemoteSDSDesignSync(req) {
		return false
	}
	return resolveSheinImageStrategy(req) == sheinImageStrategyAIGenerated
}
