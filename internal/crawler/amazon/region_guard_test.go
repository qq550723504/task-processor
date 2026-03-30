package amazon

import (
	"errors"
	"testing"
	"time"

	"task-processor/internal/core/config"
)

func TestRegionGuardOpensAndBlocksRegion(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	guard := newRegionGuard(config.AmazonRegionGuardConfig{
		Enabled:                 true,
		FailureThreshold:        2,
		EvaluationWindowSeconds: 60,
		CooldownSeconds:         30,
	})
	guard.now = func() time.Time { return now }

	if _, opened := guard.RecordFailure("us", errors.New("captcha challenge detected")); opened {
		t.Fatalf("expected first failure not to open region")
	}
	openUntil, opened := guard.RecordFailure("us", errors.New("captcha challenge detected"))
	if !opened {
		t.Fatalf("expected second failure to open region")
	}
	if !openUntil.After(now) {
		t.Fatalf("expected openUntil after now, got %v", openUntil)
	}

	blockedUntil, blocked := guard.Check("us")
	if !blocked {
		t.Fatalf("expected region to be blocked")
	}
	if !blockedUntil.Equal(openUntil) {
		t.Fatalf("expected blockedUntil=%v, got %v", openUntil, blockedUntil)
	}
}

func TestRegionGuardIgnoresNonRiskErrors(t *testing.T) {
	guard := newRegionGuard(config.AmazonRegionGuardConfig{
		Enabled:                 true,
		FailureThreshold:        1,
		EvaluationWindowSeconds: 60,
		CooldownSeconds:         30,
	})

	if _, opened := guard.RecordFailure("us", errors.New("url or asin is required")); opened {
		t.Fatalf("expected invalid request not to open region")
	}
	if _, blocked := guard.Check("us"); blocked {
		t.Fatalf("expected region to stay available")
	}
}
