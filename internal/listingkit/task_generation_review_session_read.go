package listingkit

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

	deltaToken := buildGenerationReviewReadDeltaToken(session)
	responseMode := normalizeGenerationActionResponseMode("")
	if query != nil {
		responseMode = normalizeGenerationActionResponseMode(query.ResponseMode)
	}
	if isGenerationReviewReadNotModified(query, deltaToken) {
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
