package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"task-processor/internal/batchcapture"
)

// TestReadYesIsRefusalByDefault pins design section 4 D1.4's fail-closed rule: the
// prompt is only useful if a person actually looked at the value, so a missing answer
// (including EOF from an unattended run) must never be read as confirmation.
func TestReadYesIsRefusalByDefault(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"  YES  \n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"", false},
		{"stop\n", false},
		{"y", true},
	}
	for _, tc := range cases {
		if got := readYes(strings.NewReader(tc.input)); got != tc.want {
			t.Fatalf("readYes(%q)=%v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestPromptScopeConfirmationShowsWhatTheApplicationReported is the F5-1 acceptance
// test: the value a person is asked about comes from the application, so a run whose
// flags name a different identity never gets a confirmation for the flags' value.
func TestPromptScopeConfirmationShowsWhatTheApplicationReported(t *testing.T) {
	var out bytes.Buffer
	confirm := promptScopeConfirmation(strings.NewReader("y\n"), &out)
	ok, err := confirm(batchcapture.AppScope{ActorID: "app-actor", OrganizationID: "app-org"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !ok {
		t.Fatalf("an explicit yes was not accepted")
	}
	shown := out.String()
	if !strings.Contains(shown, "app-actor") || !strings.Contains(shown, "app-org") {
		t.Fatalf("the prompt did not show the application-reported identity: %q", shown)
	}
}

func TestPromptScopeConfirmationTreatsAnEmptyAnswerAsRefusal(t *testing.T) {
	var out bytes.Buffer
	confirm := promptScopeConfirmation(strings.NewReader("\n"), &out)
	ok, err := confirm(batchcapture.AppScope{ActorID: "app-actor", OrganizationID: "app-org"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if ok {
		t.Fatalf("an empty answer was accepted as confirmation")
	}
}

// TestRepeatWhileGateNeedsAHumanRedoesTheItem is design section 4 D1.3: after a
// person clears the gate, the SAME item is re-done rather than skipped or finalised.
func TestRepeatWhileGateNeedsAHumanRedoesTheItem(t *testing.T) {
	calls := 0
	attempt := func() (batchcapture.ImportResult, error) {
		calls++
		if calls == 1 {
			return batchcapture.ImportResult{Seq: 1, Verdict: batchcapture.VerdictPauseBatch},
				fmt.Errorf("%w: challenge", batchcapture.ErrVerdictStop)
		}
		return batchcapture.ImportResult{Seq: 1, State: batchcapture.ItemSubmitted}, nil
	}
	var asked []batchcapture.ImportResult
	ask := func(r batchcapture.ImportResult, _ error) bool {
		asked = append(asked, r)
		return true
	}
	result, err := repeatWhileGateNeedsAHuman(attempt, ask)
	if err != nil {
		t.Fatalf("err=%v, want the redo to succeed", err)
	}
	if calls != 2 {
		t.Fatalf("the item was attempted %d times, want 2", calls)
	}
	if len(asked) != 1 || asked[0].Seq != 1 {
		t.Fatalf("the operator was asked about %+v, want item 1 once", asked)
	}
	if result.State != batchcapture.ItemSubmitted {
		t.Fatalf("state %q, want submitted", result.State)
	}
}

func TestRepeatWhileGateNeedsAHumanStopsWhenThePersonDeclines(t *testing.T) {
	calls := 0
	attempt := func() (batchcapture.ImportResult, error) {
		calls++
		return batchcapture.ImportResult{Seq: 3, Verdict: batchcapture.VerdictPauseBatch}, fmt.Errorf("%w: challenge", batchcapture.ErrVerdictStop)
	}
	result, err := repeatWhileGateNeedsAHuman(attempt, func(batchcapture.ImportResult, error) bool { return false })
	if err == nil {
		t.Fatalf("a declined redo returned no error")
	}
	if calls != 1 {
		t.Fatalf("the item was attempted %d times after a decline, want 1", calls)
	}
	if result.Seq != 3 {
		t.Fatalf("result %+v, want the stopped item reported back", result)
	}
}

// TestRepeatWhileGateNeedsAHumanDoesNotRedoADelistedItem guards the distinction the
// error alone cannot express: a delisted product also stops its item, but no person
// is needed, so the operator must not be asked to solve a gate that does not exist.
func TestRepeatWhileGateNeedsAHumanDoesNotRedoADelistedItem(t *testing.T) {
	calls := 0
	attempt := func() (batchcapture.ImportResult, error) {
		calls++
		return batchcapture.ImportResult{Seq: 1, State: batchcapture.ItemFailed, Verdict: batchcapture.VerdictSingleItemFailure},
			fmt.Errorf("%w: page classified as %s", batchcapture.ErrVerdictStop, batchcapture.VerdictSingleItemFailure)
	}
	asked := false
	_, _ = repeatWhileGateNeedsAHuman(attempt, func(batchcapture.ImportResult, error) bool {
		asked = true
		// Answering "no" is deliberate: if the predicate wrongly offered a redo, the
		// loop must end here so the assertion below reports the bug instead of the
		// test hanging.
		return false
	})
	if asked || calls != 1 {
		t.Fatalf("a delisted item was offered for a redo (asked=%v calls=%d)", asked, calls)
	}
}

func TestRepeatWhileGateNeedsAHumanLeavesOtherFailuresAlone(t *testing.T) {
	calls := 0
	attempt := func() (batchcapture.ImportResult, error) {
		calls++
		return batchcapture.ImportResult{Seq: 1, State: batchcapture.ItemOutcomeUnknown},
			fmt.Errorf("%w: handoff could not be observed", batchcapture.ErrOutcomeUnknown)
	}
	asked := false
	_, _ = repeatWhileGateNeedsAHuman(attempt, func(batchcapture.ImportResult, error) bool {
		asked = true
		return false
	})
	if asked || calls != 1 {
		t.Fatalf("an unknown outcome was offered for a retry (asked=%v calls=%d)", asked, calls)
	}
}

func TestWaitForGateClearedNamesTheItemAndKeepsTheBrowserOpen(t *testing.T) {
	var out bytes.Buffer
	ok := waitForGateCleared(strings.NewReader("y\n"), &out,
		batchcapture.ImportResult{Seq: 7}, fmt.Errorf("%w: page classified as pause_batch", batchcapture.ErrVerdictStop))
	if !ok {
		t.Fatalf("an explicit yes was not accepted")
	}
	shown := out.String()
	for _, want := range []string{"item 7", "browser window that is still open", "pause_batch"} {
		if !strings.Contains(shown, want) {
			t.Fatalf("the prompt does not mention %q: %q", want, shown)
		}
	}
}
