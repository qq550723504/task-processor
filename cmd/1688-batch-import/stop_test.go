package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"task-processor/internal/batchcapture"
)

// Every stop-for-a-person failure must reach exit 3 with the same instruction.
// Reporting one as a usage error (exit 2) tells the operator to fix the command
// line and run it again, which is exactly the re-run that must not happen once a
// payload may already have been delivered.
func TestDescribeStopMapsStopForAPersonFailuresToStopAndVerify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want []string
	}{
		{
			name: "blocked batch",
			err:  fmt.Errorf("%w: item 1 is submitting", batchcapture.ErrBatchBlocked),
			want: []string{"queue.json", "records which item blocked the batch", "do not re-run"},
		},
		{
			name: "unknown outcome",
			err:  fmt.Errorf("%w: no terminal result", batchcapture.ErrOutcomeUnknown),
			want: []string{"queue.json", "outcome_unknown", "do not re-run"},
		},
		{
			// The write that would have recorded the outcome failed, so the file holds
			// the earlier record. Naming outcome_unknown here would send the operator
			// looking for a label the file does not contain.
			name: "outcome unknown but the file was not updated",
			err: fmt.Errorf("%w: disk is gone; %w; %w", batchcapture.ErrQueueWriteFailed,
				batchcapture.ErrOutcomeUnknown, batchcapture.ErrItemStateUnconfirmed),
			want: []string{"queue.json", "could not be updated", "submitting", "do not re-run"},
		},
		{
			// A stop before the handoff also cannot return the item to a re-doable
			// state once the write back to queued fails, because the phase-one record
			// is still in the file.
			name: "stop before the handoff could not be recorded",
			err: fmt.Errorf("%w: disk is gone; %w", batchcapture.ErrQueueWriteFailed,
				batchcapture.ErrItemStateUnconfirmed),
			want: []string{"queue.json", "could not be updated", "submitting", "do not re-run"},
		},
		{
			// A damaged queue may already hide a submitted item: the queue is the
			// only record that an item was handed over, so an unreadable one is
			// indistinguishable from "handed over and the record was lost".
			name: "unreadable queue",
			err:  fmt.Errorf("load queue: %w", batchcapture.ErrQueueCorrupt),
			want: []string{"queue.json", "cannot be read confidently", "do not recreate the queue", "do not re-run"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeStop(tc.err, "queue.json")
			var stop *stopAndVerifyError
			if !errors.As(got, &stop) {
				t.Fatalf("err=%v, want a stopAndVerifyError so the process exits 3", got)
			}
			if !errors.Is(got, tc.err) && !errors.Is(got, errors.Unwrap(tc.err)) {
				t.Fatalf("the original failure was lost: %v", got)
			}
			for _, want := range tc.want {
				if !strings.Contains(got.Error(), want) {
					t.Fatalf("operator guidance is missing %q: %v", want, got)
				}
			}
		})
	}
}

// A scope mismatch is not ambiguous — nothing was submitted under the wrong scope
// because the run stopped before any browser work — so it must not be presented as
// a stop-and-verify case, and a successful run must stay nil.
func TestDescribeStopLeavesOtherResultsAlone(t *testing.T) {
	plain := errors.New("launch browser: boom")
	if got := describeStop(plain, "queue.json"); !errors.Is(got, plain) {
		t.Fatalf("got %v, want the original error", got)
	}
	mismatch := fmt.Errorf("queue is approved for a/b, not for c/d")
	if got := describeStop(mismatch, "queue.json"); got.Error() != mismatch.Error() {
		t.Fatalf("a scope mismatch was rewritten as %v", got)
	}
	if got := describeStop(nil, "queue.json"); got != nil {
		t.Fatalf("a successful run was turned into %v", got)
	}
}

// A queue that exists but cannot be read may already hide a submitted item, so the
// command must classify it as stop-and-verify and leave the file untouched. The
// failure has to reach describeStop even though it happens in ensureQueue, before any
// browser work — an ordinary failure there reads as "nothing to do, start over",
// which is how a damaged record turns into a duplicate publication.
//
// No browser is launched on this path, which is what makes the test runnable in CI.
func TestRunClassifiesAnUnreadableQueueAsStopAndVerify(t *testing.T) {
	cases := []struct {
		name     string
		contents func(t *testing.T, cfg config) string
	}{
		{
			name: "truncated",
			contents: func(t *testing.T, cfg config) string {
				return `{"format":"listingkit-batch-queue/1","batchID":"te`
			},
		},
		{
			// The file is valid JSON but its digest no longer matches its content,
			// so it cannot be distinguished from one that was written and then
			// partially lost.
			name: "digest mismatch",
			contents: func(t *testing.T, cfg config) string {
				queue := batchcapture.NewQueue(cfg.BatchID)
				if err := queue.ApproveScope(cfg.ActorID, cfg.Organization); err != nil {
					t.Fatalf("approve scope: %v", err)
				}
				if err := queue.Save(cfg.QueuePath); err != nil {
					t.Fatalf("save queue: %v", err)
				}
				raw, err := os.ReadFile(cfg.QueuePath)
				if err != nil {
					t.Fatalf("read queue: %v", err)
				}
				return strings.Replace(string(raw), `"`+cfg.BatchID+`"`, `"`+cfg.BatchID+`-tampered"`, 1)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			damaged := tc.contents(t, cfg)
			if err := os.WriteFile(cfg.QueuePath, []byte(damaged), 0o600); err != nil {
				t.Fatalf("write damaged queue: %v", err)
			}

			err := run(cfg.args())
			var stop *stopAndVerifyError
			if !errors.As(err, &stop) {
				t.Fatalf("err=%v, want a stopAndVerifyError so the process exits 3", err)
			}
			if !errors.Is(err, batchcapture.ErrQueueCorrupt) {
				t.Fatalf("the unreadable queue was not named as the cause: %v", err)
			}
			if !strings.Contains(err.Error(), "do not recreate the queue") {
				t.Fatalf("operator guidance is missing from %v", err)
			}
			after, readErr := os.ReadFile(cfg.QueuePath)
			if readErr != nil {
				t.Fatalf("read the queue back: %v", readErr)
			}
			if string(after) != damaged {
				t.Fatal("the command rewrote a queue it could not read, destroying the only evidence of what was submitted")
			}
		})
	}
}
