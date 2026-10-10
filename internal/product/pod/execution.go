package pod

import (
	"encoding/json"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"time"
)

const (
	StepOSS      = "oss"
	StepMaterial = "material"
	StepSync     = "sync"
)

func StepIntent(operation, step string) string { return "pod:" + operation + ":" + step }
func StepCommand(o Operation, step, owner string) (submission.AcquireExecutionCommand, error) {
	if o.Plan.Validate() != nil || step != StepOSS && step != StepMaterial && step != StepSync {
		return submission.AcquireExecutionCommand{}, ErrInvalid
	}
	input := struct {
		PlanHash    string
		Object      *ObjectReceipt
		Material    *MaterialReceipt
		PayloadHash string
	}{PlanHash: collection.Digest(o.Plan)}
	if step == StepMaterial || step == StepSync {
		if o.Object == nil {
			return submission.AcquireExecutionCommand{}, ErrInvalid
		}
		input.Object = o.Object
	}
	if step == StepSync {
		if o.Material == nil || o.Intent == nil || len(o.Payload) == 0 {
			return submission.AcquireExecutionCommand{}, ErrInvalid
		}
		input.Material = o.Material
		input.PayloadHash = collection.Digest(string(o.Payload))
	}
	raw, _ := json.Marshal(input)
	return submission.AcquireExecutionCommand{Scope: submission.ExecutionScope{OrganizationID: o.Plan.Scope.OrganizationID}, IntentKey: StepIntent(o.Plan.OperationID, step), Target: submission.ExecutionTarget{Platform: "sds", StoreID: o.Plan.Binding.MerchantID, SubjectID: o.Plan.OperationID + ":" + step}, Action: "pod_" + step, Payload: raw, ClaimOwnerID: owner, Lease: 2 * time.Minute}, nil
}
func PermitClaim(scope collection.Scope, p *submission.SendPermit) submission.ExecutionClaim {
	if p == nil {
		return submission.ExecutionClaim{}
	}
	return submission.ExecutionClaim{Scope: submission.ExecutionScope{OrganizationID: scope.OrganizationID}, AttemptID: p.AttemptID, FenceEpoch: p.FenceEpoch, OwnerID: p.ClaimOwnerID, Token: p.ClaimToken}
}

// MutationPermit exposes only the live check of the original Kernel capability
// to the provider transport. It has no durable state or independent authority.
type MutationPermit struct{ original *submission.SendPermit }

func TransportPermit(original *submission.SendPermit) *MutationPermit {
	return &MutationPermit{original: original}
}

func (p *MutationPermit) Valid() bool {
	return p != nil && p.original != nil && p.original.AttemptID != "" && p.original.ClaimToken != "" && time.Now().Before(p.original.LeaseExpiresAt)
}
