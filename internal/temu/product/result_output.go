package product

import (
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

type CommitDetailOutput struct {
	Response *temuapi.CommitDetailResponse
	Result   *temuapi.CommitDetailResult
}

type SaveProductOutput struct {
	Response *temuapi.SaveResponse
	Result   *temuapi.SaveResult
}

type SubmitProductOutput struct {
	Response     *temuapi.SubmitResponse
	SavedToDraft bool
}

func ApplyCommitDetailOutput(temuCtx *temucontext.TemuTaskContext, output *CommitDetailOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.CommitDetail = output.Response
}

func ApplySaveProductOutput(temuCtx *temucontext.TemuTaskContext, output *SaveProductOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.SaveResult = output.Response
}

func ApplySubmitProductOutput(temuCtx *temucontext.TemuTaskContext, output *SubmitProductOutput) {
	if temuCtx == nil || output == nil {
		return
	}
	temuCtx.SubmitResponse = output.Response
	temuCtx.SavedToDraft = output.SavedToDraft
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
