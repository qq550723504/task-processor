package productsourcing

import (
	"context"
	"errors"
	"sync"
	"time"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// BrowserAcquisitionService is the server-side browser provider for the same
// anonymous public contract (src2b-public-browser-v1). It differs from
// AcquisitionService in three ways, all required by the design:
//
//   - budget: the browser budget applies only to a provider sub-context, while
//     the outer budget still covers publication (D2, finding #9).
//   - ordering: it replays a durable operation (ByKey) before acquiring, so a
//     response-loss retry does not re-run the browser (D1, finding #5).
//   - admission: it never acquires for a same-key/different-offer request
//     (finding #16), a capped organization (finding #15/#19), or a second
//     concurrent first-time same-key POST (finding #10).
//
// It still owns no Product fact: only SRC-1/Catalog publication makes a fact.
type BrowserAcquisitionService struct {
	core     *AcquisitionService
	provider sourcing.PublicAcquirer
	// providerBudget bounds only the provider call; zero means the parent bound.
	providerBudget time.Duration
	// inflight serializes first-time acquisition per (organization, actor, key).
	// Entries are reference-counted and dropped when the last holder leaves, so
	// a long-lived process does not accumulate one entry per distinct key.
	inflightMu sync.Mutex
	inflight   map[string]*inflightEntry
}

// inflightEntry is a reference-counted same-key lock.
type inflightEntry struct {
	mu      sync.Mutex
	waiters int
}

// NewBrowserAcquisitionService builds the browser-backed public acquisition
// service over the same store/publisher/reader/authorizer as the HTTP path.
func NewBrowserAcquisitionService(store sourcing.AcquisitionOperationStore, provider sourcing.PublicAcquirer, publisher sourcing.AcquisitionPublisher, reader catalog.CompleteSnapshotReader, authorizer sourcing.PublicationAuthorizer, providerBudget time.Duration) (*BrowserAcquisitionService, error) {
	core, err := NewAcquisitionService(store, provider, publisher, reader, authorizer)
	if err != nil {
		return nil, err
	}
	return &BrowserAcquisitionService{core: core, provider: provider, providerBudget: providerBudget}, nil
}

// Acquire is the browser-path entrypoint for one (scope, idempotency key).
func (s *BrowserAcquisitionService) Acquire(ctx context.Context, key, source string) (sourcing.AcquisitionResult, error) {
	// Outer budget covers provider + publication; the provider gets a child
	// bounded by providerBudget (D2). Deriving the child from the parent keeps
	// publication from inheriting a browser-sized deadline.
	ctx, cancel := context.WithTimeout(ctx, s.outerBudget())
	defer cancel()
	request, err := s.core.request(ctx, key, source)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}

	// Replay first: a durable operation must be resolved without re-acquiring.
	replay, replayed, err := s.replay(ctx, request)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if replay != nil {
		return *replay, nil
	}
	if replayed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}

	// Same-key admission for a first-time request: collapse concurrent attempts
	// so only one of them can spend the shared browser/IP budget (finding #10).
	release, err := s.acquireSlot(ctx, request)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	defer release()

	// The leader may have completed while this caller waited on the same-key
	// lock. Without this re-read, a caller arriving when the organization is at
	// its last slot would fail the capacity preflight below and report
	// ACQUISITION_CAPACITY, even though a replayable operation for its own key
	// now exists. Replay it instead.
	if replay, replayed, err := s.replay(ctx, request); err != nil {
		return sourcing.AcquisitionResult{}, err
	} else if replay != nil {
		return *replay, nil
	} else if replayed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnknown
	}

	// Bounded capacity preflight so a capped organization does not launch a
	// browser for every new key (finding #15/#19). StartPrepared stays the
	// atomic correctness gate.
	if err := s.capacityAdmitted(ctx, request.Scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}

	// Provider budget is a child context only.
	providerCtx := ctx
	if s.providerBudget > 0 {
		var providerCancel context.CancelFunc
		providerCtx, providerCancel = context.WithTimeout(ctx, s.providerBudget)
		defer providerCancel()
	}
	evidence, err := s.provider.Acquire(providerCtx, request.Source)
	if err != nil {
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, providerCtx, err)
	}

	// Server-generated evidence that fails mapping is a provider/parse failure,
	// not a bad request (finding #6).
	envelope, err := sourcing.MapAcquisitionEvidence(request.Source, evidence, sourcing.AcquisitionChannelPublicBrowser, request.ID)
	if err != nil {
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, providerCtx, err)
	}
	if err := s.core.authorizeScope(ctx, request.Scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	command, err := s.core.prepareCommand(ctx, request.Scope, envelope)
	if err != nil {
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, providerCtx, err)
	}
	preparedStore, ok := s.core.operations.(sourcing.PreparedAcquisitionOperationStore)
	if !ok {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	op, claim, err := preparedStore.StartPrepared(ctx, request, command)
	if err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	if op.State == sourcing.AcquisitionFailed {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionFailed
	}
	publishClaim := false
	if op.State == sourcing.AcquisitionPrepared {
		if err := s.core.authorizeScope(ctx, op.Scope); err != nil {
			return sourcing.AcquisitionResult{}, err
		}
		op, publishClaim, err = s.core.operations.Claim(ctx, op)
		if err != nil {
			return sourcing.AcquisitionResult{}, err
		}
	}
	// A newly admitted operation is not a replay. Match the existing acquisition
	// convention (replayed := !claim) so a first-time browser acquisition is not
	// reported to clients as an idempotent replay.
	return s.core.resolve(ctx, op, publishClaim, !claim)
}

// replay resolves a durable operation without acquiring. A same-key
// different-offer request must conflict (finding #16). A prepared operation
// must be claimed before resolve, otherwise resolve rejects it as unknown and
// no scheduler will ever advance it (finding #20).
func (s *BrowserAcquisitionService) replay(ctx context.Context, request sourcing.AcquisitionOperation) (*sourcing.AcquisitionResult, bool, error) {
	op, err := s.core.operations.ByKey(ctx, request.Scope, request.Key)
	if err != nil {
		if errors.Is(err, sourcing.ErrAcquisitionNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := sameAcquisition(request, op); err != nil {
		return nil, false, err
	}
	if op.State == sourcing.AcquisitionAcquiring || op.Command == nil {
		// A durable row that exists but has no command is a terminal or in-flight
		// state left by another path (for example the HTTP provider, before the
		// collector was enabled). Re-acquiring would spend browser and shared-IP
		// capacity only for StartPrepared to rediscover the same row, so the
		// existing outcome is reported instead of launching a browser.
		if op.State == sourcing.AcquisitionFailed {
			// Surface the stored terminal failure. Reporting it as merely
			// "replayed" would make the caller answer OUTCOME_UNKNOWN instead of the
			// failure that actually happened, which is the wrong attribution for
			// every retry after a provider cutover.
			return nil, false, sourcing.ErrAcquisitionFailed
		}
		// acquiring with no command: report it as an in-flight request rather than
		// starting a competing acquisition for the same key.
		return nil, true, nil
	}
	if op.State == sourcing.AcquisitionPrepared {
		if err := s.core.authorizeScope(ctx, op.Scope); err != nil {
			return nil, false, err
		}
		claimed, publishClaim, claimErr := s.core.operations.Claim(ctx, op)
		if claimErr != nil {
			return nil, false, claimErr
		}
		if !publishClaim {
			// Another caller won the claim; surface its terminal result next read.
			return nil, true, nil
		}
		op = claimed
		// The claim token is the publication claim: a prepared operation that was
		// successfully claimed must be PUBLISHED here, not merely verified.
		// Verifying is read-only, so with no publication yet it would report
		// not-found and leave the operation stranded in publishing forever.
		result, err := s.core.resolve(ctx, op, publishClaim, true)
		if err != nil {
			return nil, false, err
		}
		return &result, true, nil
	}
	// Already publishing or published: this is a genuine replay, so the read-only
	// verification path is correct here.
	result, err := s.core.resolve(ctx, op, false, true)
	if err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

// Verify checks the original request intent and never prepares, claims, or
// writes. It delegates to the shared read/resolve contract.
func (s *BrowserAcquisitionService) Verify(ctx context.Context, key, source string) (sourcing.AcquisitionResult, error) {
	return s.core.Verify(ctx, key, source)
}

// Read returns a bounded projection of the current actor's operation.
func (s *BrowserAcquisitionService) Read(ctx context.Context, operationID string) (sourcing.AcquisitionResult, error) {
	return s.core.Read(ctx, operationID)
}

// ReadPublished returns the exact Catalog version recorded by a published
// operation after Read has bound the current actor and organization to it.
func (s *BrowserAcquisitionService) ReadPublished(ctx context.Context, operationID string) (sourcing.PublishedAcquisition, error) {
	return s.core.ReadPublished(ctx, operationID)
}

// capacityAdmitted performs a bounded preflight when the store supports it.
func (s *BrowserAcquisitionService) capacityAdmitted(ctx context.Context, scope sourcing.PublicationScope) error {
	reader, ok := s.core.operations.(sourcing.CapacityReadOperationStore)
	if !ok {
		return nil
	}
	admitted, err := reader.CapacityAdmitted(ctx, scope)
	if err != nil {
		return err
	}
	if !admitted {
		return sourcing.ErrAcquisitionCapacity
	}
	return nil
}

// acquireSlot serializes first-time acquisition for one (scope, key) so
// concurrent same-key POSTs do not each launch a browser. Correctness does not
// depend on it: StartPrepared remains the atomic arbiter. The map entry is
// reference-counted and dropped when the last holder leaves, so the map does not
// grow without bound across distinct keys over the process lifetime.
//
// The wait observes ctx, so a cancelled or expired request is not retained until
// the preceding browser call and publication finish; the outer request deadline
// therefore still bounds the handler.
func (s *BrowserAcquisitionService) acquireSlot(ctx context.Context, request sourcing.AcquisitionOperation) (func(), error) {
	key := request.Scope.OrganizationID + "|" + request.Scope.ActorID + "|" + request.Key
	s.inflightMu.Lock()
	if s.inflight == nil {
		s.inflight = make(map[string]*inflightEntry)
	}
	entry := s.inflight[key]
	if entry == nil {
		entry = &inflightEntry{}
		s.inflight[key] = entry
	}
	entry.waiters++
	s.inflightMu.Unlock()

	release := func() {
		s.inflightMu.Lock()
		entry.waiters--
		if entry.waiters <= 0 {
			delete(s.inflight, key)
		}
		s.inflightMu.Unlock()
	}

	// Cancellation-aware acquisition of the per-key lock. A plain Mutex.Lock
	// cannot observe ctx.Done(), so a timed-out request would otherwise stay
	// blocked for the whole preceding browser budget.
	acquired := make(chan struct{})
	go func() {
		entry.mu.Lock()
		close(acquired)
	}()
	select {
	case <-acquired:
		return func() {
			entry.mu.Unlock()
			release()
		}, nil
	case <-ctx.Done():
		go func() {
			<-acquired
			entry.mu.Unlock()
		}()
		release()
		return nil, ctx.Err()
	}
}

func (s *BrowserAcquisitionService) outerBudget() time.Duration {
	if s.providerBudget > 0 {
		return s.providerBudget + sourcing.AcquisitionTimeout
	}
	return sourcing.AcquisitionTimeout
}

// failFetch maps a provider/mapping failure to the established sentinels. A
// pre-admission browser failure must not leave a durable terminal row: the path
// retries by re-acquiring under the same key (D1), and nothing is written yet,
// so there is nothing to Finish.
//
// A deadline/cancellation is reported as such (the HTTP layer renders
// context.DeadlineExceeded as 504 DEADLINE_EXCEEDED); any other provider or
// mapping failure is a source failure rendered as 502 SOURCE_UNAVAILABLE, which
// is the correct "server/provider failed" projection (finding #6) and distinct
// from a 400 for a bad caller request. causeCtx is the context the provider ran
// under, so a provider-scoped timeout is not misreported as a source failure.
func (s *BrowserAcquisitionService) failFetch(parent context.Context, causeCtx context.Context, cause error) error {
	if parent != nil && parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, context.Canceled) || (causeCtx != nil && causeCtx.Err() != nil) {
		return context.DeadlineExceeded
	}
	// A collector concurrency-limit rejection is retryable, not a source
	// failure. Flattening it would report 502 SOURCE_UNAVAILABLE and, on the
	// generic service, persist the key as permanently failed.
	if errors.Is(cause, sourcing.ErrAcquisitionCapacity) {
		return sourcing.ErrAcquisitionCapacity
	}
	// A collector outage or a rejected service credential is an availability
	// failure, not a 1688 source failure. Flattening it would report
	// 502 SOURCE_UNAVAILABLE and hide the outage from retry behaviour and
	// operational monitoring.
	if errors.Is(cause, sourcing.ErrAcquisitionUnavailable) {
		return sourcing.ErrAcquisitionUnavailable
	}
	return sourcing.ErrAcquisitionFailed
}
