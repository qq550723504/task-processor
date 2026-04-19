package listingkit

func buildGenerationReviewPreviewQuery(platform, slot, capability string, preview *AssetRenderPreviewSlot) *GenerationQueueQuery {
	query := &GenerationQueueQuery{
		Platform:          platform,
		Slot:              slot,
		PreviewCapability: capability,
	}
	if preview == nil {
		return query
	}
	query.AssetID = preview.AssetID
	query.AssetRevision = preview.AssetRevision
	query.PreviewRevision = preview.PreviewRevision
	query.TaskRevision = preview.TaskRevision
	return query
}
