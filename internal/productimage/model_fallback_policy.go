package productimage

type FallbackPolicy struct {
	AllowLocalSceneFallback bool
}

func DefaultFallbackPolicy() FallbackPolicy {
	return FallbackPolicy{
		AllowLocalSceneFallback: false,
	}
}
