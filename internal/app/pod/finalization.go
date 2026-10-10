package podapp

import (
	"context"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	podstore "task-processor/internal/integration/persistence/product/pod"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"time"

	"gorm.io/gorm"
)

// The app coordinates the two current owners inside the repository's original
// Product transaction; the persistence adapter does not consume Listing rules.
func completeStep(step string, permit *submission.SendPermit, value any) podstore.Finalize {
	return func(ctx context.Context, tx *gorm.DB, current pod.Operation) error {
		if permit == nil || step != pod.StepOSS && step != pod.StepMaterial {
			return pod.ErrInvalid
		}
		finalizer, e := submissionstore.NewTransactionFinalizer(ctx, tx)
		if e != nil {
			return e
		}
		kernel, e := submission.NewExecutionKernel(finalizer)
		if e != nil {
			return e
		}
		attempt, e := exactAttempt(ctx, finalizer, current, step)
		if e != nil || attempt.AttemptID != permit.AttemptID {
			return pod.ErrUnknown
		}
		evidence := submission.ExecutionEvidence{Kind: submission.EvidenceProviderResponse, Outcome: submission.ExecutionSucceeded, Reference: current.Plan.OperationID + ":" + step, Fingerprint: collection.Digest(value), ObservedAt: time.Now().UTC()}
		_, e = kernel.Complete(ctx, pod.PermitClaim(current.Plan.Scope, permit), evidence)
		return e
	}
}

func finishExecution(q pod.QualifiedFinished) podstore.Finalize {
	return func(ctx context.Context, tx *gorm.DB, current pod.Operation) error {
		if current.Intent == nil || !q.MatchesIntent(*current.Intent) {
			return pod.ErrUnknown
		}
		finalizer, e := submissionstore.NewTransactionFinalizer(ctx, tx)
		if e != nil {
			return e
		}
		kernel, e := submission.NewExecutionKernel(finalizer)
		if e != nil {
			return e
		}
		for _, step := range []string{pod.StepOSS, pod.StepMaterial} {
			attempt, e := exactAttempt(ctx, finalizer, current, step)
			if e != nil || attempt.Status != submission.ExecutionSucceeded || attempt.Target.Platform != "sds" || attempt.Target.StoreID != current.Plan.Binding.MerchantID || attempt.Target.SubjectID != current.Plan.OperationID+":"+step {
				return pod.ErrUnknown
			}
		}
		attempt, e := exactAttempt(ctx, finalizer, current, pod.StepSync)
		if e != nil {
			return pod.ErrUnknown
		}
		if attempt.Status == submission.ExecutionClaimed {
			attempt, e = kernel.Expire(ctx, submission.ExecutionScope{OrganizationID: current.Plan.Scope.OrganizationID}, attempt.AttemptID)
			if e != nil {
				return e
			}
		}
		if attempt.Status != submission.ExecutionOutcomeUnknown {
			return pod.ErrUnknown
		}
		f := q.Reference()
		evidence := submission.ExecutionEvidence{Kind: submission.EvidenceProviderReadBack, Outcome: submission.ExecutionSucceeded, Reference: f.ID, Fingerprint: f.EvidenceDigest, ObservedAt: time.Now().UTC()}
		_, e = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: current.Plan.Scope.OrganizationID}, attempt.AttemptID, attempt.FenceEpoch, evidence)
		return e
	}
}

func exactAttempt(ctx context.Context, r *submissionstore.Repository, o pod.Operation, step string) (submission.ExecutionAttempt, error) {
	a, err := r.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: o.Plan.Scope.OrganizationID}, pod.StepIntent(o.Plan.OperationID, step))
	if err != nil {
		return a, pod.ErrUnknown
	}
	command, err := pod.StepCommand(o, step, a.ClaimOwnerID)
	if err != nil {
		return a, pod.ErrUnknown
	}
	expected, err := submission.NewExecutionReservation(command, a.AttemptID, "validation-only", a.CreatedAt)
	if err != nil || submission.ValidateExecutionReplay(a, expected.Attempt) != nil {
		return a, pod.ErrUnknown
	}
	return a, nil
}
