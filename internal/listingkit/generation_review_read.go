package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildGenerationReviewReadDeltaToken(session *GenerationReviewSession) string {
	return buildGenerationReviewDeltaToken(session)
}

func isGenerationReviewReadNotModified(query *GenerationQueueQuery, currentToken string) bool {
	if query == nil {
		return false
	}
	return listinggeneration.IsReadNotModified(query.DeltaToken, query.IfMatch, currentToken)
}

func buildGenerationReviewSessionBaseQuery(query *GenerationQueueQuery) *GenerationQueueQuery {
	if query == nil {
		return nil
	}
	base := *query
	if query.FromPlatform != "" || query.FromSlot != "" || query.FromCapability != "" || query.FromSectionKey != "" {
		base.Platform = query.FromPlatform
		base.Slot = query.FromSlot
		base.PreviewCapability = query.FromCapability
	}
	base.DeltaToken = ""
	base.IfMatch = ""
	base.ResponseMode = ""
	base.FromPlatform = ""
	base.FromSlot = ""
	base.FromCapability = ""
	base.FromSectionKey = ""
	return &base
}
