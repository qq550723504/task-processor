package listingkit

func generationWorkQueueFromPage(page *GenerationQueuePage) *GenerationWorkQueue {
	if page == nil {
		return nil
	}
	return &GenerationWorkQueue{
		Summary: page.Summary,
		Items:   append([]GenerationWorkQueueItem(nil), page.Items...),
	}
}

func generationWorkQueueFromRetryPage(page *GenerationTaskPage) *GenerationWorkQueue {
	if page == nil {
		return nil
	}
	if page.ExecutedQueue != nil {
		return page.ExecutedQueue
	}
	if page.MatchedQueue != nil {
		return page.MatchedQueue
	}
	return nil
}

func generationPreviewCapabilityLabel(capability string) string {
	switch capability {
	case "detail_preview":
		return "Detail Preview"
	case "measurement_preview":
		return "Measurement Preview"
	case "badge_preview":
		return "Badge Preview"
	case "copy_preview":
		return "Copy Preview"
	case "subject_preview":
		return "Subject Preview"
	default:
		return capability
	}
}
