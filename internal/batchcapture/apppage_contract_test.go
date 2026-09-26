package batchcapture

// The application tab is a single-page app: its URL carries the handoff key before its
// asynchronous reads have rendered the verified scope and the enabled submit control the
// executor depends on. These tests pin the readiness wait that keeps "not yet rendered"
// from being recorded as an unobservable outcome. They need no browser because the page
// read is injected, which is also what makes them run in CI.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// contractTimeoutForTest keeps the "never renders" cases short: the deadline is what
// those tests assert, and the production value is the driver's own timeout.
const (
	contractTimeoutForTest = 60 * time.Millisecond
	contractPollForTest    = time.Millisecond
)

// contractSequence answers a scripted series of page renders, repeating the last one once
// the series runs out so a "never becomes ready" case is expressed as one answer.
type contractSequence struct {
	answers []string
	calls   int
}

func (s *contractSequence) evaluate(string) (any, error) {
	index := s.calls
	s.calls++
	if index >= len(s.answers) {
		index = len(s.answers) - 1
	}
	return s.answers[index], nil
}

func contractRender(actor, organization, control, refusal string) string {
	encoded, err := json.Marshal(appContract{
		ActorID:        actor,
		OrganizationID: organization,
		Control:        control,
		Refusal:        refusal,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func TestWaitForAppContractReturnsAsSoonAsTheContractIsReady(t *testing.T) {
	page := &contractSequence{answers: []string{contractRender("actor-a", "org-a", "ready", "")}}
	if err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest); err != nil {
		t.Fatalf("waitForAppContract: %v", err)
	}
	if page.calls != 1 {
		t.Fatalf("read the page %d times, want 1: a ready page must not be re-read", page.calls)
	}
}

func TestWaitForAppContractPollsUntilTheContractRenders(t *testing.T) {
	// The sequence is the real race: nothing rendered, then only the scope, then the
	// enabled control. Only the third render may be acted on.
	page := &contractSequence{answers: []string{
		contractRender("", "", "missing", ""),
		contractRender("actor-a", "org-a", "disabled", ""),
		contractRender("actor-a", "org-a", "ready", ""),
	}}
	if err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest); err != nil {
		t.Fatalf("waitForAppContract: %v", err)
	}
	if page.calls != 3 {
		t.Fatalf("read the page %d times, want 3: the wait must keep polling until the control is enabled", page.calls)
	}
}

func TestWaitForAppContractTreatsARefusalAsAnAnswer(t *testing.T) {
	// A refusal is final and must not be waited out: ConfirmAndSubmit has to be the
	// one that reports ErrSubmitRefused (guard 3), and it can only do that if the
	// readiness wait hands the page back.
	page := &contractSequence{answers: []string{contractRender("", "", "missing", "当前用户与企业未确认，已拒绝本次操作。")}}
	if err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest); err != nil {
		t.Fatalf("waitForAppContract: %v", err)
	}
	if page.calls != 1 {
		t.Fatalf("read the page %d times, want 1: a refusal must not be waited out", page.calls)
	}
}

func TestWaitForAppContractReportsAnUnreadableScope(t *testing.T) {
	page := &contractSequence{answers: []string{contractRender("", "", "ready", "")}}
	err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("err=%v, want ErrScopeUnavailable", err)
	}
	if errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, a scope that never rendered must not be reported as a missing control", err)
	}
	if page.calls < 2 {
		t.Fatalf("read the page %d times, want the wait to have polled past the first empty render", page.calls)
	}
}

// TestWaitForAppContractWaitsForBothHalvesOfTheScope is why the predicate is not "the
// page showed something": a half-rendered scope is not a readable one (ReadAppScope
// applies IsComplete), and acting on it would report the page as unreadable instead of
// waiting the last half out.
func TestWaitForAppContractWaitsForBothHalvesOfTheScope(t *testing.T) {
	page := &contractSequence{answers: []string{
		contractRender("actor-a", "", "ready", ""),
		contractRender("actor-a", "org-a", "ready", ""),
	}}
	if err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest); err != nil {
		t.Fatalf("waitForAppContract: %v", err)
	}
	if page.calls != 2 {
		t.Fatalf("read the page %d times, want 2: a scope missing one half is not ready", page.calls)
	}
}

func TestWaitForAppContractReportsAControlThatNeverEnables(t *testing.T) {
	page := &contractSequence{answers: []string{contractRender("actor-a", "org-a", "disabled", "")}}
	err := waitForAppContract(page.evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, want ErrSubmitUnavailable", err)
	}
	if !strings.Contains(err.Error(), `control "disabled"`) {
		t.Fatalf("err=%v, want the last observed control state in the message", err)
	}
}

func TestWaitForAppContractSurfacesAnEvaluationFailure(t *testing.T) {
	evaluate := func(string) (any, error) { return nil, errors.New("page read failed: permission denied") }
	err := waitForAppContract(evaluate, contractTimeoutForTest, contractPollForTest)
	if !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, want ErrSubmitUnavailable", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("err=%v, want the underlying failure in the message", err)
	}
}

func TestWaitForAppContractReportsAPageThatNeverAnswered(t *testing.T) {
	page := &contractSequence{answers: []string{contractRender("actor-a", "org-a", "ready", "")}}
	err := waitForAppContract(page.evaluate, 0, contractPollForTest)
	if !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, want ErrSubmitUnavailable", err)
	}
	if page.calls != 0 {
		t.Fatalf("read the page %d times with an exhausted deadline, want 0", page.calls)
	}
}

// TestAppContractScriptReadsTheWholeContractInOneEvaluation pins the read to the same
// machine-readable contract ReadAppScope and ConfirmAndSubmit use. A readiness check that
// looked at rendered copy, or at only part of the contract, would let the executor act on
// a page the guards were not written for.
func TestAppContractScriptReadsTheWholeContractInOneEvaluation(t *testing.T) {
	for _, selector := range []string{selScopeActor, selScopeOrganization, selConfirmAndSubmit, selSubmitRefusal} {
		if !strings.Contains(appContractScript, selector) {
			t.Fatalf("the readiness script does not read %s", selector)
		}
	}
	if strings.Count(appContractScript, "JSON.stringify") != 1 {
		t.Fatalf("the readiness script must return the contract in a single evaluation:\n%s", appContractScript)
	}
}
