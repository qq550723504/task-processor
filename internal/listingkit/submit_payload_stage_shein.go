package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func prepareSheinSubmitPayloadProduct(
	ctx context.Context,
	taskID string,
	action string,
	requestID string,
	task *Task,
	pkg *SheinPackage,
	prepare func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error),
) (*SheinPreparedSubmitPayload, error) {
	submitProduct, err := prepare(ctx, task, pkg, action)
	if err != nil {
		return nil, err
	}
	dumpSheinSubmitPayloadForDebug(taskID, action, requestID, "prepared", submitProduct)
	return newSheinPreparedSubmitPayload(taskID, action, requestID, submitProduct), nil
}

func finalizeSheinUploadedSubmitPayload(
	ctx context.Context,
	taskID string,
	action string,
	requestID string,
	task *Task,
	in *SheinPreparedSubmitPayload,
	resolveSettings func(context.Context, *Task) SheinSettings,
) *SheinPreparedSubmitPayload {
	sheinpub.PrepareProductForSubmit(in.Product, sheinSubmitPayloadSettings(resolveSettings(ctx, task)))
	dumpSheinSubmitPayloadForDebug(taskID, action, requestID, "uploaded", in.Product)
	return refreshSheinPreparedSubmitPayloadSnapshot(in)
}
