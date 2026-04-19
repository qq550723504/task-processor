package listingkit

import "strings"

func buildGenerationReviewReadDeltaToken(session *GenerationReviewSession) string {
	return buildGenerationReviewDeltaToken(session)
}

func isGenerationReviewReadNotModified(query *GenerationQueueQuery, currentToken string) bool {
	if query == nil || strings.TrimSpace(currentToken) == "" {
		return false
	}
	if token := strings.TrimSpace(query.DeltaToken); token != "" && token == currentToken {
		return true
	}
	if token := strings.TrimSpace(query.IfMatch); token != "" && token == currentToken {
		return true
	}
	return false
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
