package listingkit

func buildGenerationNavigationDispatchResolution(focused *generationNavigationFocusedSource) *GenerationNavigationDispatchResolution {
	if focused == nil {
		return nil
	}
	return &GenerationNavigationDispatchResolution{
		SourceKind:     focused.Kind,
		SourceStep:     focused.StepIndex,
		ViaFallback:    focused.ViaFallback,
		FallbackReason: focused.FallbackReason,
	}
}
