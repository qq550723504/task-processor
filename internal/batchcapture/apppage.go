package batchcapture

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// This file drives the application capture page. Design section 4 D1.2 allows the
// executor to trigger the submission on the user's behalf, but only with four
// guards, and three of them live here:
//
//	guard 1 - the scope the submission is pinned to must be read and acted on in one
//	          uninterruptible task, so no snapshot the executor did not check can be
//	          the one the application submits with.
//	guard 2 - only real controls are clicked. The click is a DOM click on the real
//	          element, never an internal message.
//	guard 3 - a refusal by the page is final. Nothing is clicked or retried.
//
// Guard 4 (no identity actions) is structural: nothing in this file can log in,
// switch account or answer a verification challenge.
//
// The page contract is deliberately explicit rather than derived from visible text.
// The submission decision must never depend on parsing rendered copy, because the
// copy is not part of any admitted contract.

var (
	// ErrScopeUnavailable means the application did not present a readable
	// verified identity and organization. Without it no submission may happen.
	ErrScopeUnavailable = errors.New("batch capture: application scope unavailable")
	// ErrScopeMismatch means the page's scope is not the approved batch scope.
	ErrScopeMismatch = errors.New("batch capture: application scope is not the approved scope")
	// ErrSubmitRefused means the page itself declined the operation.
	ErrSubmitRefused = errors.New("batch capture: application refused the submission")
	// ErrSubmitUnavailable means the submission control was absent or the terminal
	// result never became readable.
	ErrSubmitUnavailable = errors.New("batch capture: submission control unavailable")
)

// The selectors are the capture page's machine-readable contract with the
// executor. They exist because a scope guard built on rendered prose breaks
// silently whenever the copy changes, and because design section 15.9 rejects
// "the session I happen to observe" as a substitute for an approved scope.
const (
	selScopeActor        = "[data-batch-scope-actor]"
	selScopeOrganization = "[data-batch-scope-organization]"
	selConfirmAndSubmit  = "[data-batch-confirm-submit]"
	selSubmitResult      = "[data-batch-submit-result]"
	selSubmitRefusal     = "[data-batch-submit-refusal]"
)

// AppScope is the verified identity and organization the capture page will submit
// under.
type AppScope struct {
	ActorID        string `json:"actorId"`
	OrganizationID string `json:"organizationId"`
}

// IsComplete reports whether both halves were readable. A half-read scope is
// treated as unreadable, never as a match.
func (s AppScope) IsComplete() bool { return s.ActorID != "" && s.OrganizationID != "" }

// SubmitOutcome is the terminal state the application reported for one item.
type SubmitOutcome struct {
	// Status is the page's own terminal value. It is recorded verbatim, because
	// interpreting it is the recovery branch table's job, not the driver's.
	Status string `json:"status"`
	// OperationID is present only when the page reported one.
	OperationID string `json:"operationId"`
}

// AppPage resolves the application tab a handoff opened. The caller passes the
// handoff URL rather than a page so the tab is identified by the same value the
// idempotency key came from.
func (d *Driver) AppPage(appURL string) (playwright.Page, error) {
	prefix, err := appURLPrefix(appURL)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(d.timeout)
	for time.Now().Before(deadline) {
		for _, page := range d.context.Pages() {
			if strings.HasPrefix(page.URL(), prefix) {
				return page, nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil, fmt.Errorf("%w: no tab at %s", ErrSubmitUnavailable, prefix)
}

// appURLPrefix drops the fragment so the page is found whether or not the
// application has already normalised the fragment.
func appURLPrefix(appURL string) (string, error) {
	parsed, err := url.Parse(appURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHandoffRef, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: application url has no origin", ErrHandoffRef)
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path, nil
}

// ReadAppScope reads the verified identity and organization the capture page is
// showing.
func (d *Driver) ReadAppScope(page playwright.Page) (AppScope, error) {
	raw, err := page.Evaluate(`(() => {
		const read = selector => {
			const node = document.querySelector(selector);
			return node === null ? null : (node.textContent || '').trim();
		};
		return JSON.stringify({
			actorId: read('` + selScopeActor + `'),
			organizationId: read('` + selScopeOrganization + `')
		});
	})()`)
	if err != nil {
		return AppScope{}, fmt.Errorf("%w: %v", ErrScopeUnavailable, err)
	}
	text, _ := raw.(string)
	var scope AppScope
	if err := json.Unmarshal([]byte(text), &scope); err != nil {
		return AppScope{}, fmt.Errorf("%w: %v", ErrScopeUnavailable, err)
	}
	if !scope.IsComplete() {
		return AppScope{}, fmt.Errorf("%w: page showed %+v", ErrScopeUnavailable, scope)
	}
	return scope, nil
}

// ConfirmAndSubmit clicks the capture page's real confirmation control and returns
// the terminal result the page reported.
//
// The read and the click happen inside one evaluation, which is one synchronous
// browser task with no await between them (design section 4 D1.2 guard 1). The
// approved scope is compared inside that same task, so the snapshot the decision
// uses is the snapshot in the document the click is dispatched to. Comparing
// outside would let the page change organization between the comparison and the
// click, which is precisely the window design section 15.11 records as the
// remaining risk.
func (d *Driver) ConfirmAndSubmit(page playwright.Page, approved AppScope) (SubmitOutcome, error) {
	if !approved.IsComplete() {
		return SubmitOutcome{}, fmt.Errorf("%w: approved scope is incomplete", ErrScopeMismatch)
	}
	// Guard 3: a page that already refused must never be clicked.
	refused, err := d.pageRefusal(page)
	if err != nil {
		return SubmitOutcome{}, err
	}
	if refused != "" {
		return SubmitOutcome{}, fmt.Errorf("%w: %s", ErrSubmitRefused, refused)
	}
	script := `(() => {
		const text = selector => {
			const node = document.querySelector(selector);
			return node === null ? null : (node.textContent || '').trim();
		};
		const actorId = text('` + selScopeActor + `');
		const organizationId = text('` + selScopeOrganization + `');
		const approve = ` + jsonString(approved.ActorID) + `;
		const approveOrg = ` + jsonString(approved.OrganizationID) + `;
		const control = document.querySelector('` + selConfirmAndSubmit + `');
		if (control === null) {
			return JSON.stringify({status: 'control_missing', actorId, organizationId});
		}
		if (control.disabled) {
			return JSON.stringify({status: 'control_disabled', actorId, organizationId});
		}
		if (!actorId || !organizationId) {
			return JSON.stringify({status: 'scope_unreadable', actorId, organizationId});
		}
		if (actorId !== approve || organizationId !== approveOrg) {
			return JSON.stringify({status: 'scope_mismatch', actorId, organizationId});
		}
		control.click();
		return JSON.stringify({status: 'clicked', actorId, organizationId});
	})()`
	raw, err := page.Evaluate(script)
	if err != nil {
		return SubmitOutcome{}, fmt.Errorf("%w: %v", ErrSubmitUnavailable, err)
	}
	var probe struct {
		Status         string `json:"status"`
		ActorID        string `json:"actorId"`
		OrganizationID string `json:"organizationId"`
	}
	text, _ := raw.(string)
	if err := json.Unmarshal([]byte(text), &probe); err != nil {
		return SubmitOutcome{}, fmt.Errorf("%w: %v", ErrSubmitUnavailable, err)
	}
	switch probe.Status {
	case "clicked":
	case "scope_unreadable":
		// An unreadable scope is reported as unavailable, never as a mismatch: the
		// two demand different diagnosis even though both stop the batch.
		return SubmitOutcome{}, fmt.Errorf("%w: page showed %q/%q",
			ErrScopeUnavailable, probe.ActorID, probe.OrganizationID)
	case "scope_mismatch":
		return SubmitOutcome{}, fmt.Errorf("%w: page showed %s/%s, approved %s/%s",
			ErrScopeMismatch, probe.ActorID, probe.OrganizationID, approved.ActorID, approved.OrganizationID)
	case "control_missing", "control_disabled":
		return SubmitOutcome{}, fmt.Errorf("%w: control %s", ErrSubmitUnavailable, probe.Status)
	default:
		return SubmitOutcome{}, fmt.Errorf("%w: unexpected probe %q", ErrSubmitUnavailable, probe.Status)
	}
	return d.waitForTerminalResult(page)
}

// pageRefusal reads the page's own refusal message, if any.
func (d *Driver) pageRefusal(page playwright.Page) (string, error) {
	raw, err := page.Evaluate(`(() => {
		const node = document.querySelector('` + selSubmitRefusal + `');
		return node === null ? '' : (node.textContent || '').trim();
	})()`)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSubmitUnavailable, err)
	}
	text, _ := raw.(string)
	return text, nil
}

// waitForTerminalResult waits for the capture page to report a terminal state.
//
// A readable terminal state is the only successful return. Everything else is an
// error, because design section 15.10 requires a deterministic result to be
// recorded and anything less to become `outcome_unknown` rather than be guessed.
func (d *Driver) waitForTerminalResult(page playwright.Page) (SubmitOutcome, error) {
	deadline := time.Now().Add(d.timeout)
	var last string
	for time.Now().Before(deadline) {
		raw, err := page.Evaluate(`(() => {
			const node = document.querySelector('` + selSubmitResult + `');
			if (node === null) { return ''; }
			return JSON.stringify({
				status: (node.getAttribute('data-batch-submit-result') || '').trim(),
				operationId: (node.getAttribute('data-batch-operation-id') || '').trim()
			});
		})()`)
		if err != nil {
			return SubmitOutcome{}, fmt.Errorf("%w: %v", ErrSubmitUnavailable, err)
		}
		text, _ := raw.(string)
		last = text
		var outcome SubmitOutcome
		if err := json.Unmarshal([]byte(text), &outcome); err == nil && outcome.Status != "" {
			return outcome, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return SubmitOutcome{}, fmt.Errorf("%w: no terminal result (last %q)", ErrSubmitUnavailable, last)
}

// jsonString renders one Go string as a JavaScript string literal so the approved
// scope cannot break out of the script it is embedded in.
func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		// json.Marshal cannot fail for a string; keep the failure loud rather than
		// silently substituting an empty literal that would compare as a match.
		panic(fmt.Sprintf("batch capture: encode %q: %v", value, err))
	}
	return string(encoded)
}
