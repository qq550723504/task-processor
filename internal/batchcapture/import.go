package batchcapture

import (
	"errors"
	"fmt"
)

// ImportOne drives one queued item from the local queue to a terminal result.
//
// It is the single-item slice of design section 14 S1, and its shape exists only to
// enforce the two persistence rules the design spent five review rounds getting
// right:
//
//   - The submit intent is written pessimistically BEFORE anything can be submitted
//     (section 4 D2.1). The browser is not touched until that write is durable, so a
//     crash can never leave a submitted item looking re-capturable.
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
	// Approved is the batch scope a human confirmed (design D1.4). It is compared
	// against the queue's stored scope and against the application page, so neither
	// the queue nor the live session can widen it.
	Approved AppScope
	// Persist writes the queue. It defaults to the queue's own atomic Save. It is a
	// field because "a write that is not confirmed durable must not hand off" is an
	// acceptance item, and injecting the failure is the only way to reproduce it
	// without relying on platform-specific file permissions.
	Persist func(*Queue, string) error
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
	// ErrQueueWriteFailed means the record could not be confirmed durable.
	ErrQueueWriteFailed = errors.New("batch capture: queue write not confirmed")
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
	if !queue.ScopeMatches(opts.Approved.ActorID, opts.Approved.OrganizationID) {
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

	// Phase one: record the intent to submit before the item can become submittable.
	// No key and no scope are written, because neither is knowable yet and writing a
	// value the executor has not observed is what design section 15.6 rejected.
	item.State = ItemSubmitting
	item.ApprovedActorID = opts.Approved.ActorID
	item.ApprovedOrganizationID = opts.Approved.OrganizationID
	if err := persist(queue, opts.QueuePath); err != nil {
		return ImportResult{}, fmt.Errorf("%w: %v", ErrQueueWriteFailed, err)
	}

	popup, err := driver.PrepareItem(item.URL)
	if err != nil {
		return result, stopWithoutSubmit(persist, queue, item, opts.QueuePath, fmt.Errorf("prepare item: %w", err))
	}
	defer func() { _ = popup.Close() }()

	// The page is classified from the executor's own observation, never from the
	// extension's reply, because background.ts collapses every refusal to one code.
	verdict := driver.ClassifyCurrentPage(true)
	result.Verdict = verdict
	if verdict != VerdictProceed {
		// Nothing has been handed to the application, so the item is left re-doable
		// rather than finalised: a challenge means a person must clear the gate and
		// the same item is then re-done (design sections 4 D1.3 and 7).
		item.State = ItemQueued
		item.Reason = fmt.Sprintf("page classified as %s before any handoff", verdict)
		if err := persist(queue, opts.QueuePath); err != nil {
			return result, fmt.Errorf("%w: %v", ErrQueueWriteFailed, err)
		}
		return result, fmt.Errorf("%w: %s", ErrVerdictStop, verdict)
	}

	if _, err := popup.Capture(); err != nil {
		return result, stopWithoutSubmit(persist, queue, item, opts.QueuePath, fmt.Errorf("capture: %w", err))
	}

	appURL, err := popup.Handoff()
	if err != nil {
		return result, stopWithoutSubmit(persist, queue, item, opts.QueuePath, fmt.Errorf("handoff: %w", err))
	}

	// Point of no return: the application now holds a payload for this item.
	ref, err := captureRefFromURL(appURL)
	if err != nil {
		return result, stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("read handoff key: %w", err))
	}
	item.IdempotencyKey = ref.IdempotencyKey
	result.IdempotencyKey = ref.IdempotencyKey
	if err := persist(queue, opts.QueuePath); err != nil {
		// The key could not be made durable while the payload is already visible, so
		// whether the item was published cannot be decided locally.
		return result, fmt.Errorf("%w: %v; %w", ErrQueueWriteFailed, err, ErrOutcomeUnknown)
	}

	page, err := driver.AppPage(appURL)
	if err != nil {
		return result, stopAfterHandoff(persist, queue, item, opts.QueuePath, result, fmt.Errorf("resolve application page: %w", err))
	}
	// The approved scope is compared against the page inside the same browser task
	// that clicks the control (ConfirmAndSubmit). There is deliberately no separate
	// read-then-compare here: a comparison performed in a different task from the
	// click is exactly the race this design's guard 1 exists to remove, and a second
	// check that no input can distinguish is untestable defence.
	outcome, err := driver.ConfirmAndSubmit(page, opts.Approved)
	if err != nil {
		return result, stopAfterHandoff(persist, queue, item, opts.QueuePath, result, err)
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
		return result, fmt.Errorf("%w: %v; %w", ErrQueueWriteFailed, err, ErrOutcomeUnknown)
	}
	if item.State == ItemOutcomeUnknown {
		return result, fmt.Errorf("%w: %s", ErrOutcomeUnknown, item.Reason)
	}
	return result, nil
}

// ErrVerdictStop means the batch must pause because the page was not a capturable
// product page. It is separate from ErrOutcomeUnknown: nothing was submitted, so
// the item is re-doable rather than ambiguous.
var ErrVerdictStop = errors.New("batch capture: page requires a human before continuing")

// ErrOutcomeUnknown means whether the item was published cannot be decided locally.
var ErrOutcomeUnknown = errors.New("batch capture: outcome unknown, stop and verify")

// stopWithoutSubmit leaves an item re-doable. It is only correct before a handoff,
// because before a handoff the application has never seen a payload for this item.
func stopWithoutSubmit(persist func(*Queue, string) error, queue *Queue, item *Item, path string, cause error) error {
	item.State = ItemQueued
	item.Reason = cause.Error()
	if err := persist(queue, path); err != nil {
		return fmt.Errorf("%w: %v (original: %v)", ErrQueueWriteFailed, err, cause)
	}
	return cause
}

// stopAfterHandoff records that the item's outcome cannot be decided locally. The
// item is never returned to a capturable state: a second capture would risk a
// second publication, which is the failure this package exists to prevent.
func stopAfterHandoff(persist func(*Queue, string) error, queue *Queue, item *Item, path string, result ImportResult, cause error) error {
	item.State = ItemOutcomeUnknown
	item.Reason = cause.Error()
	result.State = ItemOutcomeUnknown
	if err := persist(queue, path); err != nil {
		return fmt.Errorf("%w: %v (original: %v)", ErrQueueWriteFailed, err, cause)
	}
	return fmt.Errorf("%w: %v", ErrOutcomeUnknown, cause)
}
