package listingkit

import (
	"context"
	"fmt"
	"strings"
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

func firstSubmitReadinessMessage(readiness *SheinSubmitReadiness) string {
	if readiness == nil {
		return "SHEIN 提交前状态尚未就绪"
	}
	for _, line := range readiness.Summary {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	if len(readiness.BlockingItems) > 0 {
		return strings.TrimSpace(readiness.BlockingItems[0].Message)
	}
	return "SHEIN 提交前状态尚未就绪"
}
