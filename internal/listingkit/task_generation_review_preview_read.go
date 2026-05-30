package listingkit

type taskGenerationReviewPreviewReadPhase struct{}

func buildTaskGenerationReviewPreviewReadPhase() *taskGenerationReviewPreviewReadPhase {
	return &taskGenerationReviewPreviewReadPhase{}
}

func (p *taskGenerationReviewPreviewReadPhase) run(taskID string, snapshot *taskGenerationReviewReadSnapshot, query *GenerationQueueQuery) *GenerationReviewPreviewResponse {
	if snapshot == nil {
		snapshot = &taskGenerationReviewReadSnapshot{taskID: taskID}
	}
	if taskID == "" {
		taskID = snapshot.taskID
	}

	session := buildGenerationReviewSession(snapshot.result, snapshot.queue, query)
	if session == nil {
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{TaskID: taskID})
	}

	deltaToken := buildGenerationReviewReadDeltaToken(session)
	if isGenerationReviewReadNotModified(query, deltaToken) {
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
			TaskID:      taskID,
			DeltaToken:  deltaToken,
			NotModified: true,
		})
	}

	viewer, preview, target, toolbar := resolveGenerationReviewPreviewResponse(session, query)
	revisionStatus, revisionReason := resolveGenerationReviewPreviewRevisionStatus(viewer, query)
	return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
		TaskID:                 taskID,
		DeltaToken:             deltaToken,
		Viewer:                 viewer,
		Preview:                preview,
		ScenePreset:            buildGenerationScenePresetSummary(snapshot.result.AssetBundle, focusedPreviewAssetID(preview)),
		ReviewTarget:           target,
		Toolbar:                toolbar,
		RevisionStatus:         revisionStatus,
		RevisionMismatchReason: revisionReason,
	})
}
