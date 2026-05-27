package listingkit

func normalizeSheinPreviewPayloadSemanticFields(payload *SheinPreviewPayload) *SheinPreviewPayload {
	if payload == nil {
		return nil
	}
	if payload.DraftPayload == nil {
		payload.DraftPayload = payload.RequestDraft
	}
	payload.RequestDraft = payload.DraftPayload
	if payload.PreviewPayload == nil {
		payload.PreviewPayload = payload.PreviewProduct
	}
	payload.PreviewProduct = payload.PreviewPayload
	if payload.SubmissionState == nil {
		payload.SubmissionState = payload.Submission
	}
	payload.Submission = payload.SubmissionState
	return payload
}

func normalizeSheinExportPayloadSemanticFields(payload *SheinExportPayload) *SheinExportPayload {
	if payload == nil {
		return nil
	}
	if payload.DraftPayload == nil {
		payload.DraftPayload = payload.RequestDraft
	}
	payload.RequestDraft = payload.DraftPayload
	if payload.PreviewPayload == nil {
		payload.PreviewPayload = payload.PreviewProduct
	}
	payload.PreviewProduct = payload.PreviewPayload
	return payload
}
