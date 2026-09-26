package batchcapture

// Popup drives the extension's real action popup.
//
// Design section 4 D1.2 guard 2 requires that the executor click the extension's
// real controls. Reaching the background with an internal message would be
// simpler but would skip the popup's own two-step confirmations, so every method
// here clicks a real element in the popup document and then reads that document
// back. Nothing in this file sends an extension message itself.
//
// The popup is driven over raw CDP because Playwright does not surface it as a
// Page; driver.go records why that is necessary on Chromium 144.

import (
	"encoding/json"
	"fmt"
	"time"
)

// Selectors from extensions/1688-capture/src/popup.html. They are the popup's
// public surface: the extension's own tests and the design both refer to them.
const (
	selTitle      = "#title"
	selSource     = "#source"
	selStatus     = "#status"
	selMissing    = "#missing"
	selCapture    = "#capture"
	selHandoff    = "#handoff"
	selRefresh    = "#refresh"
	selFresh      = "#fresh"
	selConfirm    = "#new-confirm"
	selConfirmYes = "#new-confirm-button"
)

// PopupState is one consistent render of the popup document, so it reflects what
// a user would see rather than any internal executor belief.
type PopupState struct {
	Title       string `json:"title"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Missing     string `json:"missing"`
	CaptureOff  bool   `json:"captureOff"`
	HandoffOff  bool   `json:"handoffOff"`
	FreshShown  bool   `json:"freshShown"`
	ConfirmOpen bool   `json:"confirmOpen"`
}

// Captured reports whether the popup holds a capture that can be handed off.
// This is the popup's own enablement rule, read from the real document.
func (s PopupState) Captured() bool { return !s.HandoffOff }

// Popup is an attached action popup.
type Popup struct {
	raw      *rawSession
	driver   *Driver
	timeout  time.Duration
	attached bool
}

// stateScript reads every field in one evaluation so the returned state is a
// single render rather than a mix of several. It returns null until the popup's
// controls exist, which is how readiness is detected.
const stateScript = `(() => {
  const el = (id) => document.getElementById(id);
  if (!el('title') || !el('capture') || !el('handoff')) return null;
  const text = (id) => { const n = el(id); return n ? (n.textContent || '') : ''; };
  const hidden = (id) => { const n = el(id); return n ? Boolean(n.hidden) : true; };
  return {
    title: text('title'), source: text('source'), status: text('status'), missing: text('missing'),
    captureOff: Boolean(el('capture').disabled), handoffOff: Boolean(el('handoff').disabled),
    freshShown: !hidden('fresh'), confirmOpen: !hidden('new-confirm'),
  };
})()`

// State reads the current popup render.
func (p *Popup) State() (PopupState, error) {
	value, err := p.raw.evaluate(stateScript)
	if err != nil {
		return PopupState{}, err
	}
	if value == nil {
		return PopupState{}, fmt.Errorf("%w: popup document is not ready", ErrControlNotFound)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return PopupState{}, err
	}
	var state PopupState
	if err := json.Unmarshal(raw, &state); err != nil {
		return PopupState{}, fmt.Errorf("%w: unexpected popup state", ErrControlNotFound)
	}
	return state, nil
}

// WaitReady waits until the popup document has rendered its controls.
func (p *Popup) WaitReady() error {
	deadline := time.Now().Add(p.timeout)
	for time.Now().Before(deadline) {
		if _, err := p.State(); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("%w: popup never rendered %s", ErrControlNotFound, selCapture)
}

// Capture clicks the real capture control and waits for the popup to settle.
//
// A settled popup with handoff enabled means the extension produced a capture
// that passed its own validation; handoff still disabled means the capture
// failed. The reason is deliberately not reported, because it is not observable
// here: the extension collapses every capture failure to ACTION_UNAVAILABLE, so
// the executor must judge the page itself (design section 4 D1.3).
func (p *Popup) Capture() (PopupState, error) {
	before, err := p.State()
	if err != nil {
		return PopupState{}, err
	}
	if before.Captured() {
		// Already captured in this popup session. The extension treats a second
		// capture as a no-op, so clicking again would misreport that work happened.
		return before, nil
	}
	if err := p.click(selCapture); err != nil {
		return PopupState{}, err
	}
	// Confirm the click took effect before waiting for an outcome. Clicking the
	// control disables it synchronously while the extractor runs, so a capture that
	// never starts would otherwise be reported as an immediate failure.
	if err := p.waitDisabled(selCapture); err != nil {
		return PopupState{}, err
	}
	return p.waitSettled(before)
}

// waitDisabled waits until a control reports itself disabled.
//
// The script answers with a real boolean (or null when the control is absent), and
// the ANSWER is the boolean's value. Reading only the assertion's success - "the
// reply was a boolean" - is what this used to do, and it made the gate a no-op:
// false means "the control is still enabled", which is exactly the state the gate
// exists to keep waiting for, but it was accepted as proof that the click had
// engaged the control. The gate then reported a click that never started as a
// settled failure and paused a valid item.
func (p *Popup) waitDisabled(selector string) error {
	return pollControl(p.raw.evaluate, fmt.Sprintf(
		`(() => { const el = document.querySelector(%q); return el ? Boolean(el.disabled) : null; })()`, selector),
		p.timeout,
		fmt.Errorf("%w: %s was never engaged", ErrControlNotFound, selector))
}

// evaluateBool reads the answer of a popup control script. Every script here returns
// a real boolean, or null when the control is absent, so the first result is the
// value to act on and the second only reports whether a boolean came back at all.
func evaluateBool(evaluate func(string) (any, error), script string) (answer bool, ok bool, err error) {
	value, err := evaluate(script)
	if err != nil {
		return false, false, err
	}
	answer, ok = value.(bool)
	return answer, ok, nil
}

// pollControl evaluates one control script until its answer is true, and reports
// pending if it never becomes true before the timeout.
func pollControl(evaluate func(string) (any, error), script string, timeout time.Duration, pending error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		answer, ok, err := evaluateBool(evaluate, script)
		if err != nil {
			return err
		}
		// A missing control (null) answers no boolean and is still pending.
		if ok && answer {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return pending
}

// waitSettled waits until the popup reaches an outcome. The document has no busy
// flag, so the outcome is the extension's own enablement rule: either handoff
// became enabled (a capture passed the extension's validation and can be handed
// off) or the capture control became enabled again (the attempt finished without a
// usable capture). Requiring the render to repeat guards against reading a
// half-updated document.
func (p *Popup) waitSettled(before PopupState) (PopupState, error) {
	deadline := time.Now().Add(p.timeout)
	var last PopupState
	repeats := 0
	for time.Now().Before(deadline) {
		state, err := p.State()
		if err != nil {
			return PopupState{}, err
		}
		reachedOutcome := state.Captured() || !state.CaptureOff
		if state == last && reachedOutcome {
			repeats++
			if repeats >= 2 {
				return state, nil
			}
		} else {
			repeats = 0
		}
		last = state
		time.Sleep(200 * time.Millisecond)
	}
	return last, fmt.Errorf("%w: capture did not settle", ErrControlNotFound)
}

// PrepareHandoff validates that a handoff is available and records which page
// targets already exist.
//
// It deliberately does not click anything: a failure here provably happens before
// the payload leaves the executor, so the caller may still treat the item as
// re-doable.
func (p *Popup) PrepareHandoff() (map[string]bool, error) {
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	if !state.Captured() {
		return nil, fmt.Errorf("%w: handoff is not available before a successful capture", ErrNotRecapturable)
	}
	// Snapshot the tabs that already exist. The previous item's application tab is
	// normally still open and still carries the previous key, so only a tab this
	// handoff created may be adopted. A snapshot that cannot be taken is an error:
	// treating it as "no tabs" would let the previous item's tab pass as new.
	existing, err := p.driver.pageTargetIDs()
	if err != nil {
		return nil, err
	}
	return existing, nil
}

// DispatchHandoff clicks the extension's real handoff control.
//
// This click is the item's point of no return: it hands the captured payload to the
// application. An error here means the click could not be *confirmed*, not that it
// did not happen, so the caller must treat the item as possibly submitted from this
// call onwards (design section 4 D2.1).
func (p *Popup) DispatchHandoff() error {
	return p.click(selHandoff)
}

// AwaitHandoffURL returns the application URL that an already dispatched handoff
// opened. Failing to observe that URL never undoes the dispatch.
func (p *Popup) AwaitHandoffURL(existing map[string]bool) (string, error) {
	return p.driver.waitForAppTab(existing)
}

// Handoff clicks the real `在应用中导入` control and returns the URL of the app
// tab the extension created. That URL carries the idempotency key in its
// fragment, which is how the executor learns the key: the extension generates it
// and never exposes it through the popup document.
//
// Callers that must distinguish "nothing was handed over" from "handed over,
// outcome unknown" (the importer does) use the three steps above instead.
func (p *Popup) Handoff() (string, error) {
	existing, err := p.PrepareHandoff()
	if err != nil {
		return "", err
	}
	if err := p.DispatchHandoff(); err != nil {
		return "", err
	}
	return p.AwaitHandoffURL(existing)
}

// Reset starts a new capture through the popup's real two-step confirmation.
// Design section 4 D1.1 requires a per-item reset so one item's capture can never
// be handed off as another item's.
func (p *Popup) Reset() error {
	state, err := p.State()
	if err != nil {
		return err
	}
	if !state.FreshShown {
		// No capture is held in this popup session, so there is no carried-over
		// state that could be attributed to the next item.
		return nil
	}
	if err := p.click(selFresh); err != nil {
		return err
	}
	if err := p.waitVisible(selConfirm); err != nil {
		return err
	}
	if err := p.click(selConfirmYes); err != nil {
		return err
	}
	deadline := time.Now().Add(p.timeout)
	for time.Now().Before(deadline) {
		next, err := p.State()
		if err != nil {
			return err
		}
		if !next.Captured() && !next.FreshShown {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("%w: popup reset did not take effect", ErrControlNotFound)
}

// Close detaches from the popup target.
func (p *Popup) Close() error {
	if p == nil || p.raw == nil || !p.attached {
		return nil
	}
	p.attached = false
	return p.raw.close()
}

// click dispatches a real click on a real control. It is one synchronous
// evaluation so the popup decides once, and it never substitutes an internal
// message for the control.
// controlClick clicks one control and reports ErrControlNotFound unless the script
// confirmed that the click happened. The script answers false when the control is
// missing or already disabled, so the boolean's VALUE has to be checked: a boolean
// reply means the script ran, not that a click occurred.
func controlClick(evaluate func(string) (any, error), selector string) error {
	script := fmt.Sprintf(`(() => {
  const el = document.querySelector(%q);
  if (!el || el.disabled) return false;
  el.click();
  return true;
})()`, selector)
	clicked, ok, err := evaluateBool(evaluate, script)
	if err != nil {
		return err
	}
	if !ok || !clicked {
		return fmt.Errorf("%w: %s", ErrControlNotFound, selector)
	}
	return nil
}

func (p *Popup) click(selector string) error { return controlClick(p.raw.evaluate, selector) }

func (p *Popup) waitVisible(selector string) error {
	return pollControl(p.raw.evaluate, fmt.Sprintf(
		`(() => { const el = document.querySelector(%q); return el ? !el.hidden : false; })()`, selector),
		p.timeout,
		fmt.Errorf("%w: %s never became visible", ErrControlNotFound, selector))
}
