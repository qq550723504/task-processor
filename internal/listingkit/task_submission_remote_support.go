package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func newSheinRemoteSubmitService(
	executeAttempt func(context.Context, submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]) submissiondomain.RemoteSubmitResult[*sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot],
) *submissiondomain.RemoteSubmitService[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot] {
	return submissiondomain.NewRemoteSubmitService(submissiondomain.RemoteSubmitServiceConfig[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]{
		PrepareState: func(pkg *SheinPackage, action, requestID string, product *sheinproduct.Product, snapshot *sheinpub.SubmitSnapshot) (string, *sheinpub.SubmitSnapshot) {
			state := prepareSheinRemoteSubmitState(pkg, action, requestID, product, snapshot)
			return state.supplierCode, state.snapshot
		},
		ExecuteAttempt: executeAttempt,
	})
}
