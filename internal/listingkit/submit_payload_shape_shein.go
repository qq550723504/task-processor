package listingkit

import (
	"fmt"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func newSheinPreparedSubmitPayload(taskID string, action string, requestID string, product *sheinproduct.Product) *SheinPreparedSubmitPayload {
	snapshot := sheinpub.BuildSubmitSnapshot(product)
	return &SheinPreparedSubmitPayload{
		TaskID:           taskID,
		Action:           action,
		RequestID:        requestID,
		Product:          product,
		NeedsImageUpload: sheinProductPendingImageUploadCount(product) > 0,
		Snapshot:         snapshot,
	}
}

func refreshSheinPreparedSubmitPayloadSnapshot(in *SheinPreparedSubmitPayload) *SheinPreparedSubmitPayload {
	if in == nil {
		return nil
	}
	out := *in
	out.NeedsImageUpload = false
	out.Snapshot = sheinpub.BuildSubmitSnapshot(in.Product)
	return &out
}

func requireSheinPreparedSubmitPayload(in *SheinPreparedSubmitPayload) error {
	if in == nil || in.Product == nil {
		return fmt.Errorf("shein publish payload is required")
	}
	return nil
}
