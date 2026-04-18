package listingkit

import "context"

func (s *service) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	session := buildGenerationReviewSession(result, queue, query)
	if session == nil {
		return applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{TaskID: taskID}), nil
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
		}), nil
	}
	response := &GenerationReviewSessionResponse{
		TaskID:       taskID,
		DeltaToken:   deltaToken,
		ResponseMode: responseMode,
	}
	if responseMode == "patch_only" {
		baseSession := buildGenerationReviewSession(result, queue, buildGenerationReviewSessionBaseQuery(query))
		response.Patch = buildGenerationReviewSessionPatch(baseSession, session)
		if response.Patch != nil && response.Patch.DeltaToken == "" {
			response.Patch.DeltaToken = deltaToken
		}
		return applyGenerationConditionalStateToReviewSessionResponse(response), nil
	}
	response.Session = session
	return applyGenerationConditionalStateToReviewSessionResponse(response), nil
}
