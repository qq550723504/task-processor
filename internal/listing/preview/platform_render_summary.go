package preview

type PlatformRenderPreviewSummary struct {
	TotalPreviews    int
	MainAvailable    bool
	GalleryCount     int
	AuxiliaryCount   int
	CapabilityCounts map[string]int
	VisualModes      []string
}

type PlatformRenderPreviewSummaryInput[Slot any] struct {
	Main         *Slot
	Gallery      []Slot
	Auxiliary    []Slot
	VisualMode   func(Slot) string
	Capabilities func(Slot) []string
}

// SummarizePlatformRenderPreviews summarizes platform render preview slots
// without depending on legacy listing DTOs.
func SummarizePlatformRenderPreviews[Slot any](input PlatformRenderPreviewSummaryInput[Slot]) *PlatformRenderPreviewSummary {
	slots := make([]Slot, 0, 1+len(input.Gallery)+len(input.Auxiliary))
	if input.Main != nil {
		slots = append(slots, *input.Main)
	}
	slots = append(slots, input.Gallery...)
	slots = append(slots, input.Auxiliary...)
	if len(slots) == 0 {
		return nil
	}
	summary := &PlatformRenderPreviewSummary{
		TotalPreviews:  len(slots),
		MainAvailable:  input.Main != nil,
		GalleryCount:   len(input.Gallery),
		AuxiliaryCount: len(input.Auxiliary),
	}
	capabilityCounts := map[string]int{}
	visualModes := make([]string, 0, len(slots))
	for _, slot := range slots {
		if input.Capabilities != nil {
			for _, capability := range input.Capabilities(slot) {
				if capability != "" {
					capabilityCounts[capability]++
				}
			}
		}
		if input.VisualMode != nil {
			if mode := input.VisualMode(slot); mode != "" {
				visualModes = append(visualModes, mode)
			}
		}
	}
	summary.CapabilityCounts = cloneStringIntMap(capabilityCounts)
	summary.VisualModes = uniqueStrings(visualModes)
	return summary
}

func cloneStringIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]int, len(input))
	for key, value := range input {
		if key == "" || value == 0 {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
