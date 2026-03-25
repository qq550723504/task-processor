package product

import (
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

type CommitDetailOutput struct {
	Response *temuapi.CommitDetailResponse
	Result   *temuapi.CommitDetailResult
	Product  *temuapi.Product
}

type SaveProductOutput struct {
	Response *temuapi.SaveResponse
	Result   *temuapi.SaveResult
	Product  *temuapi.Product
}

type SubmitProductOutput struct {
	Response     *temuapi.SubmitResponse
	SavedToDraft bool
	Product      *temuapi.Product
}

func applyCommitDetailOutput(temuCtx *temucontext.TemuTaskContext, output *CommitDetailOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.SetCommitDetailResponse(output.Response)
	if output.Product != nil {
		temuCtx.SetPublishProductData(output.Product)
	}
}

func applySaveProductOutput(temuCtx *temucontext.TemuTaskContext, output *SaveProductOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.SetSaveResponse(output.Response)
	if output.Product != nil {
		temuCtx.SetPublishProductData(output.Product)
	}
}

func applySubmitProductOutput(temuCtx *temucontext.TemuTaskContext, output *SubmitProductOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.SetSubmitResponse(output.Response)
	temuCtx.SetSavedToDraft(output.SavedToDraft)
	if output.Product != nil {
		temuCtx.SetPublishProductData(output.Product)
	}
}

func getSubmitResponseFromContext(temuCtx *temucontext.TemuTaskContext) (*temuapi.SubmitResponse, bool) {
	if temuCtx == nil {
		return nil, false
	}
	if temuCtx.SubmitResponse != nil {
		return temuCtx.SubmitResponse, true
	}
	return nil, false
}
