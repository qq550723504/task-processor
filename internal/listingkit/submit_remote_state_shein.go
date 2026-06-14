package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinRemoteSubmitState struct {
	supplierCode string
	snapshot     *sheinpub.SubmitSnapshot
}

func prepareSheinRemoteSubmitState(pkg *SheinPackage, action string, requestID string, product *sheinproduct.Product, snapshot *sheinpub.SubmitSnapshot) sheinRemoteSubmitState {
	supplierCode, snapshot := sheinpub.PrepareSubmissionPersistenceInput(pkg, action, requestID, product, snapshot)
	return sheinRemoteSubmitState{
		supplierCode: supplierCode,
		snapshot:     snapshot,
	}
}
