package batchcapture

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// These three helpers used to read only the TYPE of the popup script's reply
// (`if ok, _ := value.(bool); ok`), which asks "was the answer a boolean?" instead of
// "what was the answer?". The scripts all answer a real boolean, so that test was
// always true: a control that was still enabled - exactly what waitDisabled exists to
// keep waiting for - was accepted as proof that the click had engaged it, and Capture
// then treated the unchanged enabled control as a settled failed capture and paused a
// valid item. click() and waitVisible() had the same defect.
//
// The helpers take their evaluate function as a parameter, so the value is asserted
// directly here rather than through a browser.

func TestEvaluateBoolReadsTheValueNotTheType(t *testing.T) {
	reply := func(value any) func(string) (any, error) {
		return func(string) (any, error) { return value, nil }
	}
	cases := []struct {
		name       string
		value      any
		wantAnswer bool
		wantOk     bool
	}{
		{"false is an answer whose value is false", false, false, true},
		{"true is an answer whose value is true", true, true, true},
		{"a missing control answers no boolean", nil, false, false},
		{"a non-boolean reply answers no boolean", "false", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer, ok, err := evaluateBool(reply(tc.value), "script")
			if err != nil {
				t.Fatalf("evaluateBool: %v", err)
			}
			if answer != tc.wantAnswer || ok != tc.wantOk {
				t.Fatalf("evaluateBool(%#v) = (%v, %v), want (%v, %v)", tc.value, answer, ok, tc.wantAnswer, tc.wantOk)
			}
		})
	}

	sentinel := errors.New("evaluate failed")
	if _, _, err := evaluateBool(func(string) (any, error) { return nil, sentinel }, "script"); !errors.Is(err, sentinel) {
		t.Fatalf("evaluateBool dropped the evaluation error: %v", err)
	}
}

// TestPollControlWaitsForTrue is the regression test for the defect: a control that
// answers false has NOT reached the state the gate is waiting for, so the gate must
// keep polling and then report its pending error rather than returning success.
func TestPollControlWaitsForTrue(t *testing.T) {
	pending := errors.New("never reached the state")

	// Always false: this is the exact case that used to be reported as success.
	calls := 0
	err := pollControl(func(string) (any, error) {
		calls++
		return false, nil
	}, "script", 20*time.Millisecond, pending)
	if !errors.Is(err, pending) {
		t.Fatalf("pollControl accepted false as the reached state (calls=%d, err=%v)", calls, err)
	}
	if calls == 0 {
		t.Fatal("pollControl never evaluated the control")
	}

	// A missing control (nil) is still pending, not success.
	if err := pollControl(func(string) (any, error) { return nil, nil }, "script", 20*time.Millisecond, pending); !errors.Is(err, pending) {
		t.Fatalf("pollControl treated a missing control as the reached state: %v", err)
	}

	// false then true: the first answer must not be accepted.
	answers := []bool{false, true}
	if err := pollControl(func(string) (any, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}, "script", 2*time.Second, pending); err != nil {
		t.Fatalf("pollControl did not return once the answer became true: %v", err)
	}

	// An evaluation error is not a pending state; it must surface immediately.
	sentinel := errors.New("evaluate failed")
	if err := pollControl(func(string) (any, error) { return nil, sentinel }, "script", time.Second, pending); !errors.Is(err, sentinel) {
		t.Fatalf("pollControl hid the evaluation error: %v", err)
	}
}

// TestControlClickRequiresTheClickToHaveHappened covers the sibling path: the click
// script answers false when the control is missing or already disabled, so a boolean
// reply means the script ran, not that a click occurred.
func TestControlClickRequiresTheClickToHaveHappened(t *testing.T) {
	if err := controlClick(func(string) (any, error) { return false, nil }, "#capture"); !errors.Is(err, ErrControlNotFound) {
		t.Fatalf("controlClick accepted false (no click happened): %v", err)
	}
	if err := controlClick(func(string) (any, error) { return nil, nil }, "#capture"); !errors.Is(err, ErrControlNotFound) {
		t.Fatalf("controlClick accepted a missing control: %v", err)
	}
	if err := controlClick(func(string) (any, error) { return "true", nil }, "#capture"); !errors.Is(err, ErrControlNotFound) {
		t.Fatalf("controlClick accepted a non-boolean reply: %v", err)
	}
	if err := controlClick(func(string) (any, error) { return true, nil }, "#capture"); err != nil {
		t.Fatalf("controlClick rejected a confirmed click: %v", err)
	}

	sentinel := errors.New("evaluate failed")
	if err := controlClick(func(string) (any, error) { return nil, sentinel }, "#capture"); !errors.Is(err, sentinel) {
		t.Fatalf("controlClick hid the evaluation error: %v", err)
	}
}

// TestPopupControlScriptsCheckTheBooleanValue pins the source so a future edit cannot
// quietly reintroduce the discarded-value assertion the three call sites all shared.
func TestPopupControlScriptsCheckTheBooleanValue(t *testing.T) {
	raw, err := os.ReadFile("popup.go")
	if err != nil {
		t.Fatalf("read popup.go: %v", err)
	}
	text := string(raw)
	for _, banned := range []string{
		"if ok, _ := value.(bool); ok {",
		"if ok, _ := value.(bool); !ok {",
	} {
		if strings.Contains(text, banned) {
			t.Fatalf("popup.go reads only the boolean's type again (%q): the answer's value is what the control gates depend on", banned)
		}
	}
	// The gate has to keep using the value-aware helper.
	if !strings.Contains(text, "return pollControl(p.raw.evaluate,") {
		t.Fatal("waitDisabled/waitVisible no longer go through pollControl")
	}
	if !strings.Contains(text, "clicked, ok, err := evaluateBool(evaluate, script)") {
		t.Fatal("click no longer goes through evaluateBool")
	}
}
