package listingkit

import (
	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func newSubmissionPayloadStageContext(taskID string, task *Task, pkg *SheinPackage, action, requestID string) submissiondomain.PayloadStageContext[*Task, *SheinPackage] {
	return submissiondomain.PayloadStageContext[*Task, *SheinPackage]{
		TaskID:    taskID,
		Task:      task,
		Package:   pkg,
		Action:    action,
		RequestID: requestID,
	}
}

func adaptListingKitPreparedPayload(in *SheinPreparedSubmitPayload) *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot] {
	if in == nil {
		return nil
	}
	return &submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		Product:          in.Product,
		NeedsImageUpload: in.NeedsImageUpload,
		Snapshot:         in.Snapshot,
	}
}

func adaptSubmissionPreparedPayload(in *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot], ctx submissiondomain.PayloadStageContext[*Task, *SheinPackage]) *SheinPreparedSubmitPayload {
	if in == nil {
		return nil
	}
	return &SheinPreparedSubmitPayload{
		TaskID:           ctx.TaskID,
		Action:           ctx.Action,
		RequestID:        ctx.RequestID,
		Product:          in.Product,
		NeedsImageUpload: in.NeedsImageUpload,
		Snapshot:         in.Snapshot,
	}
}

func newSubmissionPreparedPayload(taskID, action, requestID string, product *sheinproduct.Product) *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot] {
	return adaptListingKitPreparedPayload(newSheinPreparedSubmitPayload(taskID, action, requestID, product))
}

func requireSubmissionPreparedPayload(in *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot]) error {
	return requireSheinPreparedSubmitPayload(adaptSubmissionPreparedPayload(in, submissiondomain.PayloadStageContext[*Task, *SheinPackage]{}))
}
