package submission

import (
	"context"
	"fmt"
	"strings"
)

type ReadinessSnapshot struct {
	Ready            bool
	Summary          []string
	BlockingMessages []string
}

func FirstReadinessMessage(readiness *ReadinessSnapshot, fallback string) string {
	if readiness == nil {
		return fallback
	}
	for _, line := range readiness.Summary {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	for _, message := range readiness.BlockingMessages {
		if value := strings.TrimSpace(message); value != "" {
			return value
		}
	}
	return fallback
}

func ValidateReadinessGates[TTask, TPackage any](
	ctx context.Context,
	task TTask,
	pkg TPackage,
	action string,
	readiness *ReadinessSnapshot,
	validateFreshness func(context.Context, TTask, TPackage, string) (*ReadinessSnapshot, error),
	blockedErr error,
	fallback string,
) error {
	if readiness == nil || !readiness.Ready {
		return fmt.Errorf("%w: %s", blockedErr, FirstReadinessMessage(readiness, fallback))
	}
	if validateFreshness == nil {
		return nil
	}

	freshness, err := validateFreshness(ctx, task, pkg, action)
	if err != nil {
		return err
	}
	if freshness != nil && !freshness.Ready {
		return fmt.Errorf("%w: %s", blockedErr, FirstReadinessMessage(freshness, fallback))
	}
	return nil
}
