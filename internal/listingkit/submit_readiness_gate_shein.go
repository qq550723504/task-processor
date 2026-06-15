package listingkit

import (
	"context"

	listingsubmission "task-processor/internal/listing/submission"
)

func validateSheinSubmitReadinessGates(
	ctx context.Context,
	task *Task,
	pkg *SheinPackage,
	action string,
	readiness *SheinSubmitReadiness,
	validateFreshness func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error),
) error {
	return listingsubmission.ValidateReadinessGates(
		ctx,
		task,
		pkg,
		action,
		sheinSubmitReadinessSnapshot(readiness),
		adaptSheinSubmitFreshnessValidator(validateFreshness),
		ErrSubmitBlocked,
		"SHEIN 提交前状态尚未就绪",
	)
}

func sheinSubmitReadinessSnapshot(readiness *SheinSubmitReadiness) *listingsubmission.ReadinessSnapshot {
	if readiness == nil {
		return nil
	}
	snapshot := &listingsubmission.ReadinessSnapshot{
		Ready:   readiness.Ready,
		Summary: append([]string(nil), readiness.Summary...),
	}
	if len(readiness.BlockingItems) > 0 {
		snapshot.BlockingMessages = make([]string, 0, len(readiness.BlockingItems))
		for _, item := range readiness.BlockingItems {
			snapshot.BlockingMessages = append(snapshot.BlockingMessages, item.Message)
		}
	}
	return snapshot
}

func adaptSheinSubmitFreshnessValidator(
	validateFreshness func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error),
) func(context.Context, *Task, *SheinPackage, string) (*listingsubmission.ReadinessSnapshot, error) {
	if validateFreshness == nil {
		return nil
	}
	return func(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*listingsubmission.ReadinessSnapshot, error) {
		readiness, err := validateFreshness(ctx, task, pkg, action)
		if err != nil {
			return nil, err
		}
		return sheinSubmitReadinessSnapshot(readiness), nil
	}
}
