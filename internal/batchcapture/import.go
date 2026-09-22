package batchcapture

import (
	"errors"
	"fmt"

	"github.com/mxschmitt/playwright-go"
)

// ImportOne drives one queued item from the local queue to a terminal result.
//
// It is the single-item slice of design section 14 S1, and its shape exists only to
// enforce the two persistence rules the design spent five review rounds getting
// right:
//
//   - The submit intent is written pessimistically BEFORE the handoff click, which
//     is the only step that can make the application hold this item (section 4 D2.1).
//     The write is confirmed durable before the click, so a crash can never leave a
//     submitted item looking re-capturable. It is deliberately NOT written before
//     navigation or capture: those are local, so a crash there must leave the item
//     re-doable rather than blocked on human verification.
//   - The idempotency key joins the record as soon as it is knowable, which is when
//     the handoff returns the application URL. From that moment the payload is
//     visible to the application and a submission is possible, so every later
//     failure is recorded as outcome_unknown and never retried automatically.
//
// Errors returned after the point of no return are accompanied by an
// outcome_unknown record on disk. Callers must stop the batch on any error: the
// design deliberately has no automatic retry path.

// ImportOptions configures one item's import.
type ImportOptions struct {
	// QueuePath is the queue file to read and update.
	QueuePath string
	// Approved is the scope the operator declared for this batch, typically from
	// command-line flags. It is NOT consent: design section 4 D1.4 requires the
	// batch's authoritative scope to be read from the application and confirmed by a
	// person. Approved is only an expectation, compared against what the application
	// actually reports; a disagreement stops the batch rather than widening it.
	Approved AppScope
	// ConfirmScope is the design section 4 D1.4 consent step. It is called with the
	// identity the APPLICATION reports as verified - never with the browser profile's
	// own idea of who is signed in - and returns whether a person confirmed that the
	// batch may be attributed to it. When it is nil an unapproved queue cannot start
	// a batch at all, because there is no way to ask anyone.
	ConfirmScope func(AppScope) (bool, error)
	// Persist writes the queue. It defaults to the queue's own atomic Save. It is a
	// field because "a write that is not confirmed durable must not hand off" is an
	// acceptance item, and injecting the failure is the only way to reproduce it
	// without relying on platform-specific file permissions.
	Persist func(*Queue, string) error
	// AwaitHandoff observes the application URL a dispatched handoff opened. It
	// defaults to the popup's own implementation and exists for the same reason as
	// Persist: "the click was dispatched but its result could never be observed" is
	// the failure that decides whether an item stays re-doable, and it cannot be
	// produced deterministically through the fixture extension, which always opens
	// its application tab.
	AwaitHandoff func(*Popup, map[string]bool) (string, error)
}

// ImportResult is what one item produced.
type ImportResult struct {
	Seq            int
	URL            string
	State          ItemState
	IdempotencyKey string
	OperationID    string
	// Verdict is the page classification the item was captured under.
	Verdict Verdict
}

var (
	// ErrNoQueuedItem means the queue has nothing this executor may capture.
	ErrNoQueuedItem = errors.New("batch capture: no capturable item")
	// ErrBatchBlocked means an item's outcome cannot be decided locally, so the
	// batch must stop for a person before any further item is submitted. It is
	// distinct from ErrNoQueuedItem: "nothing can be done safely until someone
	// verifies the application" is not the same message as "there is nothing to do".
	ErrBatchBlocked = errors.New("batch capture: batch blocked, a person must verify an item first")
	// ErrQueueWriteFailed means the record could not be confirmed durable.
	ErrQueueWriteFailed = errors.New("batch capture: queue write not confirmed")
	// ErrItemStateUnconfirmed means a write failed after a handoff, so the durable
	// queue still holds the state it had BEFORE that write. That state is
	// ItemSubmitting, not the state the write intended to record, and an operator
	// reading a message that asserted the intended state would be looking at a
	// label the file does not contain.
	ErrItemStateUnconfirmed = errors.New("batch capture: durable item state may be behind")
	// ErrScopeUnconfirmed means the batch has no confirmed scope and this run could
	// not obtain one. It is deliberately not ErrScopeMismatch: nothing disagreed,
	// the consent design section 4 D1.4 requires simply was not given.
	ErrScopeUnconfirmed = errors.New("batch capture: batch scope was not confirmed")
)

// ImportOne captures and submits the first capturable item in the queue.
func ImportOne(driver *Driver, opts ImportOptions) (ImportResult, error) {
	if opts.QueuePath == "" {
		return ImportResult{}, fmt.Errorf("%w: no queue path", ErrQueueWriteFailed)
	}
	persist := opts.Persist
	if persist == nil {
		persist = func(q *Queue, path string) error { return q.Save(path) }
	}
	queue, err := Load(opts.QueuePath)
	if err != nil {
		return ImportResult{}, err
	}
	// An item that may already have been submitted must stop the batch before
	// anything else happens. Skipping over it would submit a later item while an
	// earlier one is unresolved, and a queue holding only that item would otherwise
	// be reported as "nothing to do" — which reads as safe when it is precisely the
	// case that needs a human to verify the application first (design section 4
	// D2.2, section 10 "restart" rows).
	if blocked, ok := queue.BlockingItem(); ok {
		detail := ""
		if blocked.Reason != "" {
			detail = ": " + blocked.Reason
		}
		return ImportResult{Seq: blocked.Seq, URL: blocked.URL, State: blocked.State}, fmt.Errorf(
			"%w: item %d is %s%s; verify in the application before anything else, do not re-run it",
			ErrBatchBlocked, blocked.Seq, blocked.State, detail)
	}
	// Consent (design section 4 D1.4). The queue holds the batch's only
	// authoritative scope, and it may only become approved from the identity the
	// APPLICATION reports as verified, after a person confirms it. An unapproved
	// queue is the normal state for a batch's first item: this run obtains the
	// confirmation at the moment the application first shows the identity, which is
	// after the item has been delivered to it and before the submit.
	//
	// Without a channel through which to ask, an unapproved queue must stop the batch
	// before the browser is touched. Adopting whatever identity the live session
	// happens to carry is the failure this step exists to prevent, and an unattended
	// run can never confirm anything.
	needConsent := !queue.ScopeApproved
	if needConsent {
		if opts.ConfirmScope == nil {
			return ImportResult{}, fmt.Errorf(
				"%w: queue %s carries no confirmed scope and this run cannot ask a person; nothing was submitted",
				ErrScopeUnconfirmed, opts.QueuePath)
		}
	} else if !queue.ScopeMatches(opts.Approved.ActorID, opts.Approved.OrganizationID) {
		return ImportResult{}, fmt.Errorf("%w: queue scope is %s/%s", ErrScopeMismatch,
			queue.ApprovedActorID, queue.ApprovedOrganizationID)
	}
	index := -1
	for i := range queue.Items {
		if CanRecapture(queue.Items[i].State) {
			index = i
			break
		}
	}
	if index < 0 {
		return ImportResult{}, fmt.Errorf("%w: every item is already past the point of no return", ErrNoQueuedItem)
	}
	item := &queue.Items[index]
	result := ImportResult{Seq: item.Seq, URL: item.URL}
	// previousState is the state the durable file holds for this item until phase one
	// below replaces it. Every state ImportOne is willing to start from is re-capturable,
	// so it is also the state a pre-handoff stop falls back to when a later write cannot
	// land (see stopWithoutSubmit).
	previousState := item.State

	popup, err := driver.PrepareItem(item.URL)
	if err != nil {
		return stopWithoutSubmit(persist, queue, item, opts.QueuePath, result, fmt.Errorf("prepare item: %w", err))
	}
	defer func() { _ = popup.Close() }()

	// The page is classified from the executor's own observation, never from the
	// extension's reply, because background.ts collapses every refusal to one code.
	//
	// Judgment A (a challenge or verification gate) is the one judgment that must be
	// made BEFORE a capture is triggered, because the design forbids capturing on a
	// challenge page at all. It is the same predicate Classify applies first, so the
	// two cannot disagree.
	observation := driver.ObserveCurrentPage()
	if !observation.Performed || IsChallengePage(observation.FinalURL, observation.pageEvidence()) {
		// Nothing has been handed to the application, so the item is left re-doable
		// rather than finalised: a person must clear the gate and the same item is
		// then re-done (design sections 4 D1.3 and 7).
		verdict := Classify(observation)
		result.Verdict = verdict
		return stopWithoutSubmit(persist, queue, item, opts.QueuePath, result,
			fmt.Errorf("%w: page classified as %s before any capture", ErrVerdictStop, verdict))
	}

	// Judgment B is whether the required fields were actually obtained. Capture()
	// returning nil only means the popup settled: the extension signals a refused
	// capture by leaving handoff disabled, so Captured() is the success signal.
	// Otherwise a page with no product data would be judged capturable, and a
	// definitively delisted page would never be recognised as such.
	popupState, captureErr := popup.Capture()
	observation.ProductDataPresent = captureErr == nil && popupState.Captured()
	captureNote := "capture produced no product data"
	if captureErr != nil {
		captureNote = captureErr.Error()
	}
	verdict := Classify(observation)
	result.Verdict = verdict
	switch verdict {
	case VerdictProceed:
		// Continue to the handoff below.
	case VerdictSingleItemFailure:
		// A definitively delisted product is a deterministic failure of this item
		// and, in a batch, the loop continues with the next one (design section 7).
		// No payload was ever handed to the application, so nothing is ambiguous.
		item.State = ItemFailed
		item.Reason = fmt.Sprintf("page classified as %s", verdict)
		result.State = item.State
		if err := persist(queue, opts.QueuePath); err != nil {
			return result, fmt.Errorf("%w: %v", ErrQueueWriteFailed, err)
		}
		return result, fmt.Errorf("%w: %s", ErrVerdictStop, item.Reason)
	default:
		return stopWithoutSubmit(persist, queue, item, opts.QueuePath, result, fmt.Errorf(
			"%w: page classified as %s (%s)", ErrVerdictStop, verdict, captureNote))
	}

	// The handoff is split into prepare / dispatch / await because only the dispatch
	// is the point of no return. Validating that the handoff control is usable and
	// snapshotting the existing tabs both happen before the payload can leave the
	// executor, so a failure there still leaves the item re-doable. Everything from
	// the dispatch onwards is recorded as outcome_unknown: an unconfirmed click may
	// have delivered the payload even though its result URL was never observed.
	existingTabs, err := popup.PrepareHandoff()
	if err != nil {
		return stopWithoutSubmit(persist, queue, item, opts.QueuePath, result, fmt.Errorf("handoff: %w", err))
	}

	// Phase one: record the intent to submit before the item can become submittable.
	//
	// It is written here, at the handoff boundary, rather than when the item is
	// selected. Nothing before this point can reach the application - navigation, page
	// classification and extension capture are all local - so an earlier record would
	// make a crash during those steps look like an item that might have been submitted,
	// blocking it on human verification although it provably never left the executor.
	// Design section 4 D2.1 keeps auto-recapture forbidden only for a state that may
	// have reached a submission, and its state machine is queued -> capturing ->
	// captured -> submitting: this is exactly the captured -> submitting step.
	//
	// No key and no scope are written, because neither is knowable yet and writing a
	// value the executor has not observed is what design section 15.6 rejected.
	item.State = ItemSubmitting
	if needConsent {
		// Neither value is knowable yet, and writing the flags' values would record a
		// scope no person has confirmed (design section 4 D1.4). ApproveScope fills in
		// the scope for the whole batch once there is a confirmed value to fill it with.
		item.ApprovedActorID = ""
		item.ApprovedOrganizationID = ""
	} else {
		item.ApprovedActorID = opts.Approved.ActorID
		item.ApprovedOrganizationID = opts.Approved.OrganizationID
	}
	if err := persist(queue, opts.QueuePath); err != nil {
		return phaseOneFailedWrite(item, result, err, previousState)
	}

	if err := popup.DispatchHandoff(); err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("dispatch handoff: %w", err))
	}

	// Point of no return: the application may now hold a payload for this item.
	awaitHandoff := opts.AwaitHandoff
	if awaitHandoff == nil {
		awaitHandoff = func(p *Popup, existing map[string]bool) (string, error) {
			return p.AwaitHandoffURL(existing)
		}
	}
	appURL, err := awaitHandoff(popup, existingTabs)
	if err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("handoff: %w", err))
	}
	ref, err := captureRefFromURL(appURL)
	if err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("read handoff key: %w", err))
	}
	item.IdempotencyKey = ref.IdempotencyKey
	result.IdempotencyKey = ref.IdempotencyKey
	if err := persist(queue, opts.QueuePath); err != nil {
		// The key could not be made durable while the payload is already visible, so
		// whether the item was published cannot be decided locally.
		return unconfirmedWrite(item, result, nil, err, true)
	}

	// The tab is resolved by this item's handoff key, so a leftover application tab
	// from an earlier item cannot be submitted instead (design section 4 D1.2).
	page, err := driver.AppPageByKey(ref.IdempotencyKey)
	if err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("resolve application page: %w", err))
	}
	// The tab's URL carries the key before the page has rendered the scope and submit
	// control the checks below read, so a slow render must not be recorded as an
	// unobservable outcome.
	if err := driver.waitForAppPageReady(page); err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("wait for the application page: %w", err))
	}
	// Consent is obtained here, and only here: the application is showing the item it
	// just received next to the identity it verified, so this is the first moment the
	// authoritative value exists (design section 4 D1.4 and its "取用点说明").
	approved := opts.Approved
	if needConsent {
		confirmed, err := confirmScope(driver, page, opts)
		if err != nil {
			return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, err)
		}
		if err := queue.ApproveScope(confirmed.ActorID, confirmed.OrganizationID); err != nil {
			return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("approve scope: %w", err))
		}
		// The confirmed scope must be durable before it is acted on, for the same
		// reason the key must be: the payload is already visible to the application.
		if err := persist(queue, opts.QueuePath); err != nil {
			return unconfirmedWrite(item, result, nil, err, true)
		}
		approved = confirmed
	}
	// The approved scope is compared against the page inside the same browser task
	// that clicks the control (ConfirmAndSubmit). There is deliberately no separate
	// read-then-compare here: a comparison performed in a different task from the
	// click is exactly the race this design's guard 1 exists to remove, and a second
	// check that no input can distinguish is untestable defence.
	outcome, err := driver.ConfirmAndSubmit(page, approved)
	if err != nil {
		return stopAfterHandoff(persist, queue, item, opts.QueuePath, result, err)
	}
	// A terminal result was read back, which design section 15.10 requires to be
	// recorded rather than replaced by an unknown.
	item.OperationID = outcome.OperationID
	result.OperationID = outcome.OperationID
	switch outcome.Status {
	case "published":
		item.State = ItemSubmitted
	case "failed":
		item.State = ItemFailed
		item.Reason = "application reported a terminal failure"
	default:
		// A status the executor does not recognise is not a terminal result.
		item.State = ItemOutcomeUnknown
		item.Reason = fmt.Sprintf("unrecognised terminal status %q", outcome.Status)
	}
	result.State = item.State
	if err := persist(queue, opts.QueuePath); err != nil {
		return unconfirmedWrite(item, result, nil, err, true)
	}
	if item.State == ItemOutcomeUnknown {
		return result, fmt.Errorf("%w: %s", ErrOutcomeUnknown, item.Reason)
	}
	return result, nil
}

// confirmScope reads the server-verified identity from the application page and has
// a person confirm it (design section 4 D1.4 step 1 and 2).
//
// The value comes from the application, never from the browser profile or the flags;
// the flags are only the operator's declared expectation and are compared against
// what the application reports. A disagreement stops the batch, because only a
// person can say which of the two is right, and neither may be assumed.
func confirmScope(driver *Driver, page playwright.Page, opts ImportOptions) (AppScope, error) {
	observed, err := driver.ReadAppScope(page)
	if err != nil {
		return AppScope{}, fmt.Errorf("read the identity the application verified: %w", err)
	}
	if observed != opts.Approved {
		return AppScope{}, fmt.Errorf(
			"%w: the application reports %s/%s but this run was started for %s/%s",
			ErrScopeMismatch, observed.ActorID, observed.OrganizationID, opts.Approved.ActorID, opts.Approved.OrganizationID)
	}
	confirmed, err := opts.ConfirmScope(observed)
	if err != nil {
		return AppScope{}, fmt.Errorf("ask for scope confirmation: %w", err)
	}
	if !confirmed {
		return AppScope{}, fmt.Errorf(
			"%w: the identity the application reported (%s/%s) was not confirmed, so nothing was attributed to it",
			ErrScopeUnconfirmed, observed.ActorID, observed.OrganizationID)
	}
	return observed, nil
}

// ErrVerdictStop means the batch must pause because the page was not a capturable
// product page. It is separate from ErrOutcomeUnknown: nothing was submitted, so
// the item is re-doable rather than ambiguous.
var ErrVerdictStop = errors.New("batch capture: page requires a human before continuing")

// NeedsVisibleSession reports whether the failure is resolved by a person acting
// inside the browser the executor opened - a login wall, a verification redirect or
// a risk-control page (design section 4 D1.3).
//
// Such a stop is the one case where closing the browser destroys the thing the
// operator needs: the session they must sign in or clear a verification page in.
// The item is still re-capturable at that point (nothing was handed to the
// application), so the correct continuation is to keep the window open and redo the
// item once the person has dealt with it.
//
// The verdict is required, not just the error: a definitively delisted product also
// stops its item with ErrVerdictStop, and no person is needed for it. VerdictAction
// already carries that distinction, so it is not re-derived here.
func NeedsVisibleSession(result ImportResult, err error) bool {
	return errors.Is(err, ErrVerdictStop) && result.Verdict.Action().RedoCurrentItemAfterHuman
}

// unconfirmedWrite reports a persist that could not be confirmed durable, and returns
// the state the durable queue actually holds.
//
// Save replaces the file atomically and writes the previous content back when a flush
// fails after the replacement, so an unconfirmed write leaves the PREVIOUS record in
// place. After phase 1 that record says ItemSubmitting, whatever the failed write
// intended to say (queued, outcome_unknown, submitted, failed). Reporting the
// intended state would name a label the operator cannot find in the file and would
// hide the one thing that has to be verified: which of the two records survived.
//
// outcomeUnknown adds ErrOutcomeUnknown for the paths where the payload may already
// have been visible to the application; it is not added for a stop that happened
// before the handoff, because then no external side effect can exist.
func unconfirmedWrite(item *Item, result ImportResult, cause, writeErr error, outcomeUnknown bool) (ImportResult, error) {
	reason := "the queue write could not be confirmed"
	if cause != nil {
		reason = cause.Error()
	}
	item.State = ItemSubmitting
	item.Reason = reason
	result.State = ItemSubmitting
	tail := fmt.Errorf("%w: the queue file may still record %q; verify in the application before doing anything else", ErrItemStateUnconfirmed, item.State)
	// The cause is deliberately not wrapped with %w. A wrapped cause would keep
	// matching ErrVerdictStop, and NeedsVisibleSession reads that as "clear the gate and
	// redo the item" — which is not available while the file still says submitting.
	if outcomeUnknown {
		return result, fmt.Errorf("%w: %v (original: %v); %w; %w", ErrQueueWriteFailed, writeErr, cause, ErrOutcomeUnknown, tail)
	}
	return result, fmt.Errorf("%w: %v (original: %v); %w", ErrQueueWriteFailed, writeErr, cause, tail)
}

// ErrOutcomeUnknown means whether the item was published cannot be decided locally.
var ErrOutcomeUnknown = errors.New("batch capture: outcome unknown, stop and verify")

// phaseOneFailedWrite classifies a failed phase-one write, which is the record that makes
// the item submittable and therefore the last thing that happens before the dispatch.
//
// Fail-closed is the easy half: the dispatch below is only allowed once this record is
// durable, so a failed write stops the item before the application can see anything.
// Because the dispatch never happened the item is still re-capturable and the file still
// holds previousState - an ordinary re-runnable failure, never the outcome_unknown the
// post-dispatch paths have to report.
//
// The exception is a save that replaced the file and then could not flush it: the file
// then holds ItemSubmitting, which is the one state that forbids an automatic redo. An
// operator told "nothing was handed off, so the item can be re-done" would follow that
// advice, be stopped by BlockingItem, and be unable to reconcile the two - so that case
// reports the unconfirmed state instead (design section 4 D2.2, AGENTS.md's rule that a
// finding may not be answered with a message the file contradicts).
func phaseOneFailedWrite(item *Item, result ImportResult, err error, previousState ItemState) (ImportResult, error) {
	if errors.Is(err, errQueueContentUnrestored) {
		// Nothing was handed off, so this is deliberately NOT ErrOutcomeUnknown: the
		// application provably never saw a payload. What is unconfirmed is the item's
		// state in the file.
		return unconfirmedWrite(item, result, nil, err, false)
	}
	item.State = previousState
	item.Reason = ""
	result.State = previousState
	return result, fmt.Errorf("%w: %v (nothing was handed off, so the item can be re-done)", ErrQueueWriteFailed, err)
}

// stopWithoutSubmit leaves an item re-doable. It is only correct before a handoff,
// because before a handoff the application has never seen a payload for this item.
//
// It returns the result so the caller reports the state the queue now holds: returning
// the pre-stop value would print an empty state while the queue says queued, which is
// exactly the kind of mismatch an operator cannot reconcile during recovery.
func stopWithoutSubmit(persist func(*Queue, string) error, queue *Queue, item *Item, path string, result ImportResult, cause error) (ImportResult, error) {
	// The item never reached the handoff, so phase one has not run: nothing has made it
	// un-recapturable and the file still holds a state ImportOne was willing to start
	// from. This write therefore only records the reason, and the item stays re-doable
	// even if it does not land - a failed revert is an ordinary failure, not the
	// "the file may say submitting" case the post-handoff paths must report.
	previous := item.State
	item.State = ItemQueued
	item.Reason = cause.Error()
	result.State = ItemQueued
	if err := persist(queue, path); err != nil {
		if errors.Is(err, errQueueContentUnrestored) {
			// The file may hold either state, and both are re-capturable, so the item can
			// still be re-done - but the message must not claim to know which one is in the
			// file. The cause stays wrapped so NeedsVisibleSession still recognises a gate
			// stop: the operator can clear it in the still-open browser and redo this item.
			return result, fmt.Errorf(
				"%w: %v; %q was never handed off and the queue file holds a re-capturable state (%q or %q), so the item can be re-done once the cause is cleared; %w",
				ErrQueueWriteFailed, err, item.URL, previous, ItemQueued, cause)
		}
		item.State = previous
		item.Reason = ""
		result.State = previous
		// The cause stays wrapped so NeedsVisibleSession still recognises a gate stop:
		// the operator can clear it in the still-open browser and redo this item.
		return result, fmt.Errorf(
			"%w: %v; %q was never handed off and the file still records it as %q, so the item can be re-done once the cause is cleared; %w",
			ErrQueueWriteFailed, err, item.URL, previous, cause)
	}
	return result, cause
}

// stopAfterHandoff records that the item's outcome cannot be decided locally. The
// item is never returned to a capturable state: a second capture would risk a
// second publication, which is the failure this package exists to prevent.
//
// Like stopWithoutSubmit it returns the result rather than only the error, so the
// caller cannot report a state that disagrees with the durable queue.
func stopAfterHandoff(persist func(*Queue, string) error, queue *Queue, item *Item, path string, result ImportResult, cause error) (ImportResult, error) {
	item.State = ItemOutcomeUnknown
	item.Reason = cause.Error()
	result.State = ItemOutcomeUnknown
	if err := persist(queue, path); err != nil {
		// Both errors matter: the write failure explains the tooling problem, and the
		// unknown outcome is what forbids an automatic retry. Reporting only the write
		// failure lets a caller read the run as an ordinary failure and try again.
		return unconfirmedWrite(item, result, cause, err, true)
	}
	// The cause stays wrapped as well as the outcome classification, because the two
	// answer different questions: ErrOutcomeUnknown is what forbids an automatic
	// retry, and the cause (a scope refusal, an unobservable handoff) is what the
	// operator has to act on.
	return result, fmt.Errorf("%w: %w", ErrOutcomeUnknown, cause)
}
