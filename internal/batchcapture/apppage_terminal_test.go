package batchcapture

// The terminal wait is the second half of guard 3. ConfirmAndSubmit refuses to click a page
// that already said "nothing was dispatched", but the page gives that same answer after the
// click too: a submitting attempt can lose its context, exhaust capacity or pass its
// deadline. These tests pin that the wait ends on the page's own refusal rather than
// spending the whole driver timeout and recording a generic transfer failure.
//
// The wait does not know which statuses are terminal - a non-empty status with an operation
// id is the page's word that it reached one, and the page owns that list, because it owns
// its own result contract. What the wait does own is that an unreadable render is not a
// result, and that a refusal outranks a result.
//
// They need no browser because the page read is injected, which is also what makes them run
// in CI.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// submitRender is the JSON the page read returns. The keys are written out literally
// rather than marshalled from submitProbe, so a rename on the Go side alone cannot make
// this test agree with a page read that no longer parses.
func submitRender(status, operationID, refusal string) string {
	return `{"status":"` + status + `","operationId":"` + operationID + `","refusal":"` + refusal + `"}`
}

const submitOperationID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"

func TestWaitForSubmitResultReadsTheRefusalChannelInTheSameSnapshot(t *testing.T) {
	// Both terminal channels must come from one evaluation. Reading them separately would let
	// a page be observed as refusing and as having published in two different reads, which is
	// exactly the drift the refusal check exists to catch. The read is asserted on the script
	// the wait passes to the page, because that script is the wait's own behaviour: these
	// tests inject the page, so nothing else can observe what was asked of it.
	captured := ""
	evaluate := func(script string) (any, error) {
		captured = script
		return submitRender("published", submitOperationID, ""), nil
	}
	if _, err := waitForSubmitResult(evaluate, contractTimeoutForTest, contractPollForTest); err != nil {
		t.Fatalf("waitForSubmitResult: %v", err)
	}
	for _, want := range []string{selSubmitResult, selSubmitRefusal, "refusal.textContent"} {
		if !strings.Contains(captured, want) {
			t.Fatalf("the terminal read never asks the page for %q:\n%s", want, captured)
		}
	}
}

func TestWaitForSubmitResultReturnsTheTerminalResult(t *testing.T) {
	page := &contractSequence{answers: []string{submitRender("published", submitOperationID, "")}}
	outcome, err := waitForSubmitResult(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if err != nil {
		t.Fatalf("waitForSubmitResult: %v", err)
	}
	if outcome.Status != "published" || outcome.OperationID != submitOperationID {
		t.Fatalf("read %+v, want the published operation", outcome)
	}
	// The refusal is in the same snapshot; a page that published must not be re-read.
	if page.calls != 1 {
		t.Fatalf("read the page %d times, want 1", page.calls)
	}
}

func TestWaitForSubmitResultEndsOnARefusalWithoutWaitingOutTheTimeout(t *testing.T) {
	// The refusal is the page's final answer. Waiting out the deadline would record an
	// unobservable handoff for an item the page already explained, and would do it after
	// the driver's full timeout per item.
	page := &contractSequence{answers: []string{submitRender("", "", "CONTEXT_UNAVAILABLE")}}
	_, err := waitForSubmitResult(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("got %v, want ErrSubmitRefused", err)
	}
	if !strings.Contains(err.Error(), "CONTEXT_UNAVAILABLE") {
		t.Fatalf("the refusal code must survive into the error, got %q", err.Error())
	}
	if page.calls != 1 {
		t.Fatalf("read the page %d times, want 1: a refusal must not be waited out", page.calls)
	}
}

func TestWaitForSubmitResultStopsOnARefusalThatArrivesAfterAnEmptyRead(t *testing.T) {
	// The real sequence: the click has happened, the page has rendered the result node but
	// not filled it in, and then the attempt fails before dispatch. The refusal has to win,
	// and the empty render must not be read as an answer.
	page := &contractSequence{answers: []string{
		submitRender("", "", ""),
		submitRender("", "", "DEADLINE_EXCEEDED"),
	}}
	_, err := waitForSubmitResult(page.evaluate, time.Second, contractPollForTest)
	if !errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("got %v, want ErrSubmitRefused", err)
	}
	if !strings.Contains(err.Error(), "DEADLINE_EXCEEDED") {
		t.Fatalf("the refusal code must survive into the error, got %q", err.Error())
	}
	if page.calls != 2 {
		t.Fatalf("read the page %d times, want 2", page.calls)
	}
}

func TestWaitForSubmitResultRefusesToPublishOverARefusal(t *testing.T) {
	// Both channels should be mutually exclusive per attempt, but a drifted page can render
	// both. The refusal is the answer that stops a person: returning the result would claim
	// an operation the page says it never dispatched.
	page := &contractSequence{answers: []string{submitRender("published", submitOperationID, "NOT_DISPATCHED")}}
	outcome, err := waitForSubmitResult(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("got %v, want ErrSubmitRefused", err)
	}
	if outcome.Status != "" || outcome.OperationID != "" {
		t.Fatalf("returned %+v alongside a refusal, want no result", outcome)
	}
}

func TestWaitForSubmitResultPollsUntilTheResultIsReadable(t *testing.T) {
	// The capture page renders the result node as soon as it mounts, with no status and no
	// operation id, so an unreadable render is the normal first read, not an error.
	page := &contractSequence{answers: []string{
		submitRender("", "", ""),
		submitRender("published", "", ""),
		submitRender("published", submitOperationID, ""),
	}}
	outcome, err := waitForSubmitResult(page.evaluate, time.Second, contractPollForTest)
	if err != nil {
		t.Fatalf("waitForSubmitResult: %v", err)
	}
	if outcome.Status != "published" {
		t.Fatalf("read %+v, want the published operation", outcome)
	}
	if page.calls != 3 {
		t.Fatalf("read the page %d times, want 3: only a status with an operation id is an answer", page.calls)
	}
}

func TestWaitForSubmitResultRejectsAStatusWithoutAnOperationID(t *testing.T) {
	// A named status with no operation id is a partial or drifted render. Recording it would
	// claim a publication nobody can look up.
	page := &contractSequence{answers: []string{submitRender("published", "", "")}}
	_, err := waitForSubmitResult(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("got %v, want ErrSubmitUnavailable", err)
	}
	if errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("an unreadable result is not a refusal: %v", err)
	}
}

func TestWaitForSubmitResultReportsAFailedReadAsUnavailable(t *testing.T) {
	evaluate := func(string) (any, error) { return nil, errors.New("detached frame") }
	_, err := waitForSubmitResult(evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("got %v, want ErrSubmitUnavailable", err)
	}
	if !strings.Contains(err.Error(), "detached frame") {
		t.Fatalf("the read failure must survive into the error, got %q", err.Error())
	}
}
