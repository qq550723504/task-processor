package listingkit

import (
	"context"
	"fmt"
)

func validateSheinSubmitReadinessGates(
	ctx context.Context,
	task *Task,
	pkg *SheinPackage,
	action string,
	readiness *SheinSubmitReadiness,
	validateFreshness func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error),
) error {
	if readiness == nil || !readiness.Ready {
		return fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))
	}
	if validateFreshness == nil {
		return nil
	}

	freshness, err := validateFreshness(ctx, task, pkg, action)
	if err != nil {
		return err
	}
	if freshness != nil && !freshness.Ready {
		return fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(freshness))
	}
	return nil
}
