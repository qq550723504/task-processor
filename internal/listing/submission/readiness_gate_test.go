package submission

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateReadinessGatesBlocksOnBaseReadiness(t *testing.T) {
	t.Parallel()

	err := ValidateReadinessGates(
		context.Background(),
		"task-1",
		"pkg-1",
		"publish",
		&ReadinessSnapshot{
			Ready:   false,
			Summary: []string{"still blocked"},
		},
		nil,
		errors.New("blocked"),
		"fallback message",
	)
	if err == nil || !strings.Contains(err.Error(), "still blocked") {
		t.Fatalf("err = %v, want blocked summary", err)
	}
}

func TestValidateReadinessGatesBlocksOnFreshnessReadiness(t *testing.T) {
	t.Parallel()

	err := ValidateReadinessGates(
		context.Background(),
		"task-1",
		"pkg-1",
		"publish",
		&ReadinessSnapshot{Ready: true},
		func(context.Context, string, string, string) (*ReadinessSnapshot, error) {
			return &ReadinessSnapshot{
				Ready:            false,
				BlockingMessages: []string{"freshness drift"},
			}, nil
		},
		errors.New("blocked"),
		"fallback message",
	)
	if err == nil || !strings.Contains(err.Error(), "freshness drift") {
		t.Fatalf("err = %v, want freshness blocker", err)
	}
}

func TestFirstReadinessMessageFallsBack(t *testing.T) {
	t.Parallel()

	if got := FirstReadinessMessage(nil, "fallback"); got != "fallback" {
		t.Fatalf("FirstReadinessMessage(nil) = %q, want fallback", got)
	}
	if got := FirstReadinessMessage(&ReadinessSnapshot{}, "fallback"); got != "fallback" {
		t.Fatalf("FirstReadinessMessage(empty) = %q, want fallback", got)
	}
}
