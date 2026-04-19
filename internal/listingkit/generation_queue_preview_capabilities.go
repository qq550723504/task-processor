package listingkit

func buildRenderPreviewCapabilities(item GenerationWorkQueueItem) []string {
	out := make([]string, 0, 5)
	for _, layerType := range item.RenderPreviewLayerTypes {
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
