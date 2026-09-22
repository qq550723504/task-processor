// Package batchcapture implements the executor-local state required to drive the
// existing 1688 browser-capture chain for a batch of product links.
//
// It owns exactly one durable fact: "what did I intend to do, and how far did I
// get". It is deliberately NOT a second source of truth for publication results —
// those stay in the server-side acquisition operation (see the design doc
// docs/superpowers/specs/2026-09-21-1688-batch-local-agent-design.md, sections
// D2 and D2.2).
//
// Two safety properties are load-bearing and are enforced here rather than left
// to callers:
//
//  1. Only an item that is known never to have been submitted may be re-captured.
//     An item that may have been submitted can never silently become re-capturable;
//     it is reported as outcome_unknown and the batch stops for human review.
//  2. The queue file is written atomically and flushed to stable storage before a
//     handoff is allowed to happen, and any unreadable/corrupt/truncated content is
//     rejected wholesale instead of being partially trusted. That flush covers every
//     directory the write created, not just the one holding the file: a directory's
//     name is made durable by flushing its parent.
//
// Neither property is optional: dropping the first causes duplicate publication,
// dropping the second causes duplicate publication after a power loss.
package batchcapture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ItemState is the executor-local progress of one link. It mirrors no server
// state machine; the server only ever sees an acquisition operation that was
// actually submitted.
type ItemState string

const (
	// ItemQueued means the link is known but nothing has happened yet.
	ItemQueued ItemState = "queued"
	// ItemCapturing means the page is being driven; nothing has been submitted.
	ItemCapturing ItemState = "capturing"
	// ItemCaptured means an envelope exists in the extension and, at most, a
	// handoff page was opened. No POST has been attempted from this item, so
	// re-doing the item is safe.
	ItemCaptured ItemState = "captured"
	// ItemSubmitting means a submission may already have been dispatched. It is
	// written BEFORE the handoff so that a crash can never lose it. It is the
	// state this package exists to get right: see CanRecapture.
	ItemSubmitting ItemState = "submitting"
	// ItemSubmitted means a terminal server result was read back for this item.
	ItemSubmitted ItemState = "submitted"
	// ItemFailed means a definitive, deterministic source-side failure (delisted
	// or invalid link). It is not a challenge page; see the design's D1.3.
	ItemFailed ItemState = "failed"
	// ItemOutcomeUnknown means the executor cannot decide whether this item was
	// published. The only allowed action is to stop and ask a human.
	ItemOutcomeUnknown ItemState = "outcome_unknown"
)

// QueueFormat is the on-disk schema identifier. It is written so that a future
// format change is detected instead of being misparsed.
const QueueFormat = "listingkit.1688.batch-queue/v1"

var (
	// ErrQueueCorrupt reports content that exists but cannot be trusted. Callers
	// must stop the batch; they must never treat the affected items as new.
	ErrQueueCorrupt = errors.New("batch queue is corrupt or unreadable")
	// ErrQueueMissing reports that no queue file exists yet.
	ErrQueueMissing = errors.New("batch queue does not exist")
	// ErrNoApprovedScope reports that the batch has no user-confirmed scope.
	ErrNoApprovedScope = errors.New("batch has no approved scope")
	// ErrNotRecapturable reports an attempt to re-capture an item that may
	// already have been submitted.
	ErrNotRecapturable = errors.New("item may already have been submitted")
)

// Item is one link and the executor-local facts known about it.
type Item struct {
	Seq    int       `json:"seq"`
	URL    string    `json:"url"`
	State  ItemState `json:"state"`
	Reason string    `json:"reason,omitempty"`

	// ApprovedActorID/ApprovedOrganizationID are copied from the batch scope once
	// the user has confirmed it (design D1.4). They are never derived from the
	// browser session at submission time.
	ApprovedActorID        string `json:"approvedActorId,omitempty"`
	ApprovedOrganizationID string `json:"approvedOrganizationId,omitempty"`

	// IdempotencyKey is the extension-generated key read back from the handoff
	// URL. It is a read-back handle, not a retry credential: the executor cannot
	// choose or reuse it (design D2).
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	OperationID    string `json:"operationId,omitempty"`
}

// Queue is the whole durable batch intent.
type Queue struct {
	Format  string `json:"format"`
	BatchID string `json:"batchId"`
	// ApprovedActorID/ApprovedOrganizationID are the batch scope. They stay empty
	// until a human confirms the value read from the application, which is what
	// makes "observed session state" different from "consent" (design D1.4).
	ApprovedActorID        string `json:"approvedActorId,omitempty"`
	ApprovedOrganizationID string `json:"approvedOrganizationId,omitempty"`
	ScopeApproved          bool   `json:"scopeApproved"`
	Items                  []Item `json:"items"`

	// digest authenticates Items/scope. It is computed on save and verified on
	// load so that truncation or tampering is rejected rather than partially
	// trusted.
	digest string
}

// envelope is the exact bytes that are written to disk and hashed.
type envelope struct {
	Format                 string `json:"format"`
	BatchID                string `json:"batchId"`
	ApprovedActorID        string `json:"approvedActorId,omitempty"`
	ApprovedOrganizationID string `json:"approvedOrganizationId,omitempty"`
	ScopeApproved          bool   `json:"scopeApproved"`
	Items                  []Item `json:"items"`
	Digest                 string `json:"digest"`
}

// NewQueue returns an empty queue with an unapproved scope. A caller must not
// start a batch from this queue until ApproveScope has been called.
func NewQueue(batchID string) *Queue {
	return &Queue{Format: QueueFormat, BatchID: batchID}
}

// ApproveScope binds the batch to an actor and organization that a human has
// explicitly confirmed. It is the only way ScopeApproved becomes true, so a
// batch can never inherit consent from whatever the session happened to be.
func (q *Queue) ApproveScope(actorID, organizationID string) error {
	if actorID == "" || organizationID == "" {
		return fmt.Errorf("approve scope: actor and organization are required")
	}
	q.ApprovedActorID = actorID
	q.ApprovedOrganizationID = organizationID
	q.ScopeApproved = true
	for i := range q.Items {
		q.Items[i].ApprovedActorID = actorID
		q.Items[i].ApprovedOrganizationID = organizationID
	}
	return nil
}

// ScopeMatches reports whether the observed session identity is the scope a
// human approved. A false result must stop the batch; it is never a reason to
// adopt the observed value.
func (q *Queue) ScopeMatches(actorID, organizationID string) bool {
	if !q.ScopeApproved {
		return false
	}
	return q.ApprovedActorID == actorID && q.ApprovedOrganizationID == organizationID
}

// CanRecapture reports whether re-doing this item is safe.
//
// This is the single most important rule in the package. It is deliberately not
// "the item is not terminal": an item in ItemSubmitting may already have been
// published, so re-doing it could publish a second time. Only states that
// provably never reached a submission are re-capturable.
func CanRecapture(state ItemState) bool {
	switch state {
	case ItemQueued, ItemCapturing, ItemCaptured:
		return true
	default:
		return false
	}
}

// RequiresHumanReview reports whether the item's outcome cannot be decided
// locally and the batch must stop for a person.
func RequiresHumanReview(state ItemState) bool {
	return state == ItemSubmitting || state == ItemOutcomeUnknown
}

// BlockingItem returns the first item that requires human review, if any.
func (q *Queue) BlockingItem() (Item, bool) {
	for _, item := range q.Items {
		if RequiresHumanReview(item.State) {
			return item, true
		}
	}
	return Item{}, false
}

// Validate reports whether the queue is internally consistent. It is used both
// before starting a batch and right after loading, so that inconsistent state
// stops the batch instead of driving it.
func (q *Queue) Validate() error {
	if q.Format != QueueFormat {
		return fmt.Errorf("%w: unexpected format %q", ErrQueueCorrupt, q.Format)
	}
	if q.ScopeApproved && (q.ApprovedActorID == "" || q.ApprovedOrganizationID == "") {
		return fmt.Errorf("%w: scope marked approved but values are empty", ErrQueueCorrupt)
	}
	// The converse is equally inconsistent and must fail closed: a populated
	// scope without the approval flag is the shape a hand-edited or partially
	// recovered file would take, and treating it as usable is exactly how an
	// observed session state could get mistaken for consent.
	if !q.ScopeApproved && (q.ApprovedActorID != "" || q.ApprovedOrganizationID != "") {
		return fmt.Errorf("%w: scope values present but not approved", ErrQueueCorrupt)
	}
	seen := make(map[int]bool, len(q.Items))
	for _, item := range q.Items {
		if item.URL == "" {
			return fmt.Errorf("%w: item %d has no url", ErrQueueCorrupt, item.Seq)
		}
		if seen[item.Seq] {
			return fmt.Errorf("%w: duplicate seq %d", ErrQueueCorrupt, item.Seq)
		}
		seen[item.Seq] = true
		if !validState(item.State) {
			return fmt.Errorf("%w: item %d has unknown state %q", ErrQueueCorrupt, item.Seq, item.State)
		}
		// An item that may have been submitted must carry the scope it was
		// submitted under, otherwise a restart cannot even ask the right
		// question when reading the result back.
		if item.State == ItemSubmitting && q.ScopeApproved {
			if item.ApprovedActorID != q.ApprovedActorID || item.ApprovedOrganizationID != q.ApprovedOrganizationID {
				return fmt.Errorf("%w: item %d carries a different scope than the batch", ErrQueueCorrupt, item.Seq)
			}
		}
	}
	return nil
}

func validState(state ItemState) bool {
	switch state {
	case ItemQueued, ItemCapturing, ItemCaptured, ItemSubmitting,
		ItemSubmitted, ItemFailed, ItemOutcomeUnknown:
		return true
	default:
		return false
	}
}

// Load reads and authenticates a queue file.
//
// Any unreadable, truncated, malformed or tampered content returns
// ErrQueueCorrupt and no queue. That is deliberate: partial trust of a damaged
// file is how a previously-submitted item gets mistaken for a fresh one.
func Load(path string) (*Queue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrQueueMissing
		}
		return nil, fmt.Errorf("%w: %v", ErrQueueCorrupt, err)
	}
	var env envelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueCorrupt, err)
	}
	// Trailing content after the object means the file is not what it claims.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing content", ErrQueueCorrupt)
	}
	env.Digest = ""
	expected, err := digestOf(env)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueCorrupt, err)
	}
	var onDisk envelope
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueCorrupt, err)
	}
	if onDisk.Digest == "" || onDisk.Digest != expected {
		return nil, fmt.Errorf("%w: digest mismatch", ErrQueueCorrupt)
	}
	queue := &Queue{
		Format:                 env.Format,
		BatchID:                env.BatchID,
		ApprovedActorID:        env.ApprovedActorID,
		ApprovedOrganizationID: env.ApprovedOrganizationID,
		ScopeApproved:          env.ScopeApproved,
		Items:                  env.Items,
		digest:                 expected,
	}
	if queue.Items == nil {
		queue.Items = []Item{}
	}
	if err := queue.Validate(); err != nil {
		return nil, err
	}
	return queue, nil
}

// Save writes the queue atomically and flushes it to stable storage.
//
// The sequence is: write a sibling temp file, flush its contents, atomically
// replace the target, flush the target's directory, and flush the parent of every
// directory this call created. On the platforms this executor runs on, os.Rename
// replaces an existing file (MoveFileEx with MOVEFILE_REPLACE_EXISTING on Windows,
// rename(2) elsewhere), so a reader never observes a half-written queue.
//
// A non-nil error means the state is NOT durable. Callers must treat that as
// fatal for the current step: in particular, the handoff must not be performed
// unless the pre-write succeeded.
//
// A non-nil error also means the file still holds the content it held before this
// call. The replace is atomic, but it becomes durable only once the directories
// involved are flushed, and a flush can fail after the replacement is already
// visible; the previous content is then written back so that a caller which reports
// "nothing changed, the item can be re-done" is describing the file and not just its
// own intent. A save that cannot do that returns an error carrying
// errQueueContentUnrestored instead, and callers must not report the previous state
// as the file's contents in that case.
func (q *Queue) Save(path string) error { return q.save(path, syncDir) }

// save carries Save's body with its directory flush injected, so the durability
// contract can be asserted without a power loss. Production only ever calls it
// through Save with syncDir.
func (q *Queue) save(path string, syncDirFn func(string) error) error {
	if err := q.Validate(); err != nil {
		return err
	}
	env := envelope{
		Format:                 QueueFormat,
		BatchID:                q.BatchID,
		ApprovedActorID:        q.ApprovedActorID,
		ApprovedOrganizationID: q.ApprovedOrganizationID,
		ScopeApproved:          q.ScopeApproved,
		Items:                  q.Items,
	}
	if env.Items == nil {
		env.Items = []Item{}
	}
	digest, err := digestOf(env)
	if err != nil {
		return err
	}
	env.Digest = digest
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	q.Format = QueueFormat
	q.digest = digest

	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	createdDirs, err := mkdirQueueDir(dir)
	if err != nil {
		return fmt.Errorf("create queue directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".batch-queue-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp queue file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup; after a successful rename this is a no-op.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp queue file: %w", err)
	}
	// Flush before the atomic replace, otherwise a crash can expose a renamed
	// but not-yet-persisted file (the precise failure this protocol exists to
	// prevent).
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush temp queue file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp queue file: %w", err)
	}
	// The content the file holds now, so a failure after the replacement below can put
	// it back. A read that fails for any reason other than "no file yet" does not abort
	// the save - a file whose contents cannot be read is still allowed to be replaced -
	// it only means the previous content cannot be restored, which is reported if the
	// replacement turns out to need it.
	previous, previousErr := os.ReadFile(path)
	if err := replaceFile(tmpName, path); err != nil {
		return fmt.Errorf("replace queue file: %w", err)
	}
	// The replace is only durable once its directory entry is flushed, so a
	// failure here must not be reported as a successful save: the caller uses this
	// return value to decide whether the item may become submittable. On Windows the
	// flush happens inside replaceFile (MOVEFILE_WRITE_THROUGH) and this call is a
	// no-op; see sync_dir_windows.go.
	if err := syncDirFn(dir); err != nil {
		return flushFailedAfterReplace(path, previous, previousErr, syncDirFn, fmt.Errorf("flush queue directory: %w", err))
	}
	// Flushing a directory registers its CONTENTS; the directory's own NAME lives in
	// its parent, so every directory this call created is still unregistered until
	// that parent is flushed as well. Without this, a power loss after a save that
	// returned success can remove the whole queue directory, and the next run would
	// then read no queue at all (ErrQueueMissing), rebuild the item as queued, and be
	// willing to publish it a second time - the duplicate publication the durable
	// pre-write exists to prevent. The list is shallowest-first so each parent is
	// registered before the child it names.
	for _, created := range createdDirs {
		if err := syncDirFn(filepath.Dir(created)); err != nil {
			return flushFailedAfterReplace(path, previous, previousErr, syncDirFn, fmt.Errorf("flush parent of newly created queue directory: %w", err))
		}
	}
	return nil
}

// errQueueContentUnrestored marks a save that failed after its replacement had already
// become visible and could not put the previous content back. The queue file may hold
// the content that save wrote, so a caller may not report the state it meant to replace
// as the one the file still holds; see phaseOneFailedWrite in import.go.
var errQueueContentUnrestored = errors.New("queue file content could not be restored")

// flushFailedAfterReplace handles a save that failed after its replacement became
// visible. Flushing a directory registers the file's directory entry, so until that
// succeeds the new content is visible but not guaranteed to survive a crash, while the
// callers of Save treat a failed save as "the file still holds the previous state".
// Writing the previous content back makes that assumption true again.
//
// If the restore fails as well the file may hold the content this save wrote. Returning
// no error is not an option (the new content is not durable), and reporting the previous
// state would describe a record the file does not contain - for the pre-handoff intent
// write that state is a re-capturable one, so the operator would be sent to redo an item
// the file records as possibly submitted.
func flushFailedAfterReplace(path string, previous []byte, previousErr error, syncDirFn func(string) error, flushErr error) error {
	if restoreErr := restoreQueueFile(path, previous, previousErr, syncDirFn); restoreErr != nil {
		return fmt.Errorf("%w: %w (and the previous content could not be restored: %v)",
			errQueueContentUnrestored, flushErr, restoreErr)
	}
	return flushErr
}

// restoreQueueFile puts back the content the queue file held before a replacement whose
// durability could not be confirmed, using the same durable sequence Save uses.
func restoreQueueFile(path string, previous []byte, previousErr error, syncDirFn func(string) error) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	switch {
	case previousErr == nil:
		// Fall through to rewriting the previous content below.
	case errors.Is(previousErr, os.ErrNotExist):
		// The file did not exist before this save, so removing what the save created is
		// as close to the previous state as a restore can get.
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove queue file whose replacement was not confirmed: %w", err)
		}
		if err := syncDirFn(dir); err != nil {
			return fmt.Errorf("flush queue directory after removing it: %w", err)
		}
		return nil
	default:
		// The file exists but its content is unknown. Removing it would lose a queue, so
		// there is nothing safe to do here.
		return fmt.Errorf("read the previous queue file: %v", previousErr)
	}
	tmp, err := os.CreateTemp(dir, ".batch-queue-restore-*.tmp")
	if err != nil {
		return fmt.Errorf("create restore temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup; after a successful rename this is a no-op.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(previous); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write restore temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush restore temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close restore temp file: %w", err)
	}
	if err := replaceFile(tmpName, path); err != nil {
		return fmt.Errorf("replace queue file with its previous content: %w", err)
	}
	if err := syncDirFn(dir); err != nil {
		return fmt.Errorf("flush queue directory after restoring it: %w", err)
	}
	return nil
}

// mkdirQueueDir creates dir and any missing parents, and returns the directories it
// actually created, ordered shallowest-first.
//
// Save needs that list because a directory's contents are made durable by flushing
// the directory, while its NAME is made durable by flushing its parent. Flushing
// only dir therefore leaves every newly created ancestor unregistered, which is a
// silent durability hole exactly when the operator points --queue at a path whose
// directories do not exist yet.
func mkdirQueueDir(dir string) ([]string, error) {
	var created []string
	for p := dir; ; {
		if _, err := os.Stat(p); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		created = append(created, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Collected deepest-first while walking up; the caller must flush the
	// shallowest parent first.
	for i, j := 0, len(created)-1; i < j; i, j = i+1, j-1 {
		created[i], created[j] = created[j], created[i]
	}
	return created, nil
}

func digestOf(env envelope) (string, error) {
	env.Digest = ""
	payload, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
