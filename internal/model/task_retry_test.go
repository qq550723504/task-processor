package model

import "testing"

func TestResolveTaskMaxRetries(t *testing.T) {
	cases := []struct {
		name       string
		configured int
		want       int
	}{
		{name: "positive", configured: 5, want: 5},
		{name: "zero defaults", configured: 0, want: 3},
		{name: "negative defaults", configured: -1, want: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveTaskMaxRetries(tc.configured); got != tc.want {
				t.Fatalf("ResolveTaskMaxRetries(%d) = %d, want %d", tc.configured, got, tc.want)
			}
		})
	}
}

func TestApplyRetryFailure(t *testing.T) {
	task := &Task{
		RetryCount: 0,
		Priority:   25,
	}

	decision := ApplyRetryFailure(task, 3)
	if decision.Exhausted {
		t.Fatal("first retry should not be exhausted")
	}
	if decision.OriginalPriority != 25 || decision.CurrentPriority != 15 {
		t.Fatalf("priority = %d -> %d, want 25 -> 15", decision.OriginalPriority, decision.CurrentPriority)
	}
	if decision.RetryCount != 1 || task.RetryCount != 1 {
		t.Fatalf("retry count = %d/%d, want 1", decision.RetryCount, task.RetryCount)
	}
}

func TestApplyRetryFailureExhausted(t *testing.T) {
	task := &Task{
		RetryCount: 2,
		Priority:   8,
	}

	decision := ApplyRetryFailure(task, 3)
	if !decision.Exhausted {
		t.Fatal("third retry should be exhausted")
	}
	if decision.CurrentPriority != 8 {
		t.Fatalf("priority should stay 8, got %d", decision.CurrentPriority)
	}
}
