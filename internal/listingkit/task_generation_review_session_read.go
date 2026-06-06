package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

type taskGenerationReviewSessionReadPhase struct{}

func buildTaskGenerationReviewSessionReadPhase() *taskGenerationReviewSessionReadPhase {
	return &taskGenerationReviewSessionReadPhase{}
}

func (p *taskGenerationReviewSessionReadPhase) run(taskID string, snapshot *taskGenerationReviewReadSnapshot, query *GenerationQueueQuery) *GenerationReviewSessionResponse {
	if snapshot == nil {
		snapshot = &taskGenerationReviewReadSnapshot{taskID: taskID}
	}
	if taskID == "" {
		taskID = snapshot.taskID
	}

	session := buildGenerationReviewSession(snapshot.result, snapshot.queue, query)
	if session == nil {
		return applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{TaskID: taskID})
	}

	deltaToken := buildGenerationReviewDeltaToken(session)
	responseMode := normalizeGenerationActionResponseMode("")
	if query != nil {
		responseMode = normalizeGenerationActionResponseMode(query.ResponseMode)
	}
	if query != nil && listinggeneration.IsReadNotModified(query.DeltaToken, query.IfMatch, deltaToken) {
		return applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{
			TaskID:       taskID,
			DeltaToken:   deltaToken,
			NotModified:  true,
			ResponseMode: responseMode,
		})
	}

	response := &GenerationReviewSessionResponse{
		TaskID:       taskID,
		DeltaToken:   deltaToken,
		ResponseMode: responseMode,
	}
	if responseMode == "patch_only" {
		baseSession := buildGenerationReviewSession(snapshot.result, snapshot.queue, buildGenerationReviewSessionBaseQuery(query))
		response.Patch = buildGenerationReviewSessionPatch(baseSession, session)
		if response.Patch != nil && response.Patch.DeltaToken == "" {
			response.Patch.DeltaToken = deltaToken
		}
		return applyGenerationConditionalStateToReviewSessionResponse(response)
	}

	response.Session = session
	return applyGenerationConditionalStateToReviewSessionResponse(response)
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
