package listingkit

func applyGenerationRecoverySummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {
	if page == nil {
		return nil
	}
	page.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(page.ResourceDescriptors)
	return page
}

func applyGenerationRecoverySummaryToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {
	if result == nil {
		return nil
	}
	result.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(result.ResourceDescriptors)
	return result
}
