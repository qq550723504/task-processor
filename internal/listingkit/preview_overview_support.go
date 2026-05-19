package listingkit

import "strings"

func normalizePreviewPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func buildRevisionHistoryPreviewItems(records []ListingKitRevisionRecord) []ListingKitRevisionRecord {
	if len(records) == 0 {
		return nil
	}
	items := make([]ListingKitRevisionRecord, 0, len(records))
	for i, record := range records {
		items = append(items, withRevisionHistoryRecordID(record, i))
	}
	return items
}

func previewPlatforms(task *Task) []string {
	if task == nil {
		return nil
	}
	if task.Result != nil && len(task.Result.Platforms) > 0 {
		return append([]string(nil), task.Result.Platforms...)
	}
	if task.Request != nil && len(task.Request.Platforms) > 0 {
		return append([]string(nil), task.Request.Platforms...)
	}
	return nil
}

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	if result == nil {
		return nil
	}

	header := &ListingKitPreviewHeader{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
	}
	if result.Summary != nil {
		header.SourceType = result.Summary.SourceType
		header.ImageCount = result.Summary.ImageCount
		header.VariantCount = result.Summary.VariantCount
		header.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	header.ReviewReasons = reviewReasonsFromResult(result)
	header.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return header
}
