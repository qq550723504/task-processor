package preview

// RenderPreviewCapabilities maps render layer types to review capabilities.
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
	return uniqueStrings(out)
}

// RenderPreviewCapabilitiesForSlot maps slot render metadata to preview
// capabilities, falling back to subject preview for raster-only previews.
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
