package extractor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/mxschmitt/playwright-go"
)

type extractorFunc func(playwright.Page, *model.Product1688) error

func (f extractorFunc) Extract(page playwright.Page, product *model.Product1688) error {
	return f(page, product)
}

func TestWaitForContextReturnsCancellationPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := waitForContext(ctx, 5*time.Second)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForContext() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("waitForContext() took %v after cancellation", elapsed)
	}
}

func TestRunExtractorsStopsBeforeLaterStageAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var laterStageCalls atomic.Int32
	extractors := []BaseExtractor{
		extractorFunc(func(playwright.Page, *model.Product1688) error {
			cancel()
			return nil
		}),
		extractorFunc(func(playwright.Page, *model.Product1688) error {
			laterStageCalls.Add(1)
			return nil
		}),
	}

	err := runExtractors(ctx, nil, &model.Product1688{}, extractors)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runExtractors() error = %v, want context.Canceled", err)
	}
	if got := laterStageCalls.Load(); got != 0 {
		t.Fatalf("later extractor stages started %d times, want 0", got)
	}
}
