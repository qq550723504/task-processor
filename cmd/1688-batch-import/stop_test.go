package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"task-processor/internal/batchcapture"
)

// A blocked batch and an unknown outcome are the same instruction to the operator —
// stop and check the application — and both must reach exit 3. Reporting either as a
// usage error (exit 2) tells the operator to fix the command line and re-run it,
// which is exactly the re-run that must not happen once a payload may have been
// delivered.
func TestDescribeStopMapsABlockedBatchToStopAndVerify(t *testing.T) {
	err := describeStop(fmt.Errorf("%w: item 1 is submitting", batchcapture.ErrBatchBlocked), "queue.json")
	var blocked *batchBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("err=%v, want a batchBlockedError so the process exits 3", err)
	}
	var unknown *outcomeUnknownError
	if errors.As(err, &unknown) {
		t.Fatalf("a blocked batch was reported as an unknown outcome: %v", err)
	}
	for _, want := range []string{"queue.json", "do not re-run"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("operator guidance is missing %q: %v", want, err)
		}
	}
}

func TestDescribeStopMapsAnUnknownOutcomeToStopAndVerify(t *testing.T) {
	err := describeStop(fmt.Errorf("%w: no terminal result", batchcapture.ErrOutcomeUnknown), "queue.json")
	var unknown *outcomeUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("err=%v, want an outcomeUnknownError so the process exits 3", err)
	}
	if !strings.Contains(err.Error(), "outcome_unknown") {
		t.Fatalf("the state the queue recorded is not named: %v", err)
	}
}

// Every other failure keeps its own error so it lands on exit 2. A successful run
// must stay nil.
func TestDescribeStopPassesOtherResultsThrough(t *testing.T) {
	plain := errors.New("launch browser: boom")
	if got := describeStop(plain, "queue.json"); !errors.Is(got, plain) {
		t.Fatalf("got %v, want the original error", got)
	}
	if got := describeStop(nil, "queue.json"); got != nil {
		t.Fatalf("a successful run was turned into %v", got)
	}
}
