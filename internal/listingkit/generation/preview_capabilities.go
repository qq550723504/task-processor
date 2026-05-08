package generation

func RenderPreviewCapabilities(layerTypes []string) []string {
	out := make([]string, 0, 5)
	for _, layerType := range layerTypes {
		switch layerType {
		case "badge":
			out = append(out, "badge_preview")
		case "spec":
			out = append(out, "measurement_preview")
		case "detail":
			out = append(out, "detail_preview")
		case "text":
			out = append(out, "copy_preview")
		case "subject":
			out = append(out, "subject_preview")
		}
	}
	if len(out) == 0 {
		return nil
	}
	return uniqueStrings(out)
}

func RenderPreviewCapabilitiesForSlot(layerTypes []string, previewSVG, assetURL string) []string {
	capabilities := RenderPreviewCapabilities(layerTypes)
	if len(capabilities) > 0 {
		return capabilities
	}
	if previewSVG == "" && assetURL != "" {
		return []string{"subject_preview"}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
