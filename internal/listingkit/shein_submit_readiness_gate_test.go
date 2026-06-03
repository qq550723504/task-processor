package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateSheinSubmitReadinessGatesBlocksOnBaseReadiness(t *testing.T) {
	t.Parallel()

	err := validateSheinSubmitReadinessGates(
		context.Background(),
		makeReadySheinTask(),
		makeReadySheinTask().Result.Shein,
		"publish",
		&SheinSubmitReadiness{
			Ready:  false,
			Status: "blocked",
			Summary: []string{
				"当前还有关键字段未完成",
			},
		},
		nil,
	)
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("err = %v, want ErrSubmitBlocked", err)
	}
	if !strings.Contains(err.Error(), "当前还有关键字段未完成") {
		t.Fatalf("err = %v, want base readiness message", err)
	}
}

func TestValidateSheinSubmitReadinessGatesBlocksOnFreshnessReadiness(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	err := validateSheinSubmitReadinessGates(
		context.Background(),
		task,
		task.Result.Shein,
		"publish",
		&SheinSubmitReadiness{Ready: true, Status: "ready"},
		func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error) {
			return &SheinSubmitReadiness{
				Ready:  false,
				Status: "blocked",
				BlockingItems: []SheinReadinessItem{{
					Key:     sheinFreshnessCategoryKey,
					Label:   "类目模板新鲜度",
					Message: "当前类目模板已发生变化",
				}},
			}, nil
		},
	)
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("err = %v, want ErrSubmitBlocked", err)
	}
	if !strings.Contains(err.Error(), "当前类目模板已发生变化") {
		t.Fatalf("err = %v, want freshness readiness message", err)
	}
}
