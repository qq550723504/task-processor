package listingkit

import "context"

func (s *service) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {
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
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{TaskID: taskID}), nil
	}
	deltaToken := buildGenerationReviewReadDeltaToken(session)
	if isGenerationReviewReadNotModified(query, deltaToken) {
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
			TaskID:      taskID,
			DeltaToken:  deltaToken,
			NotModified: true,
		}), nil
	}
	viewer, preview, target, toolbar := resolveGenerationReviewPreviewResponse(session, query)
	revisionStatus, revisionReason := resolveGenerationReviewPreviewRevisionStatus(viewer, query)
	return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
		TaskID:                 taskID,
		DeltaToken:             deltaToken,
		Viewer:                 viewer,
		Preview:                preview,
		ReviewTarget:           target,
		Toolbar:                toolbar,
		RevisionStatus:         revisionStatus,
		RevisionMismatchReason: revisionReason,
	}), nil
}

func (in *GenerationReviewToolbarInput) GetViewer() *GenerationReviewPreviewViewer {
	if in == nil {
		return nil
	}
	return in.PreviewViewer
}
