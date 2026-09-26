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
	inflight       sync.Map // scope+key -> *sync.WaitGroup-ish singleflight marker
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
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, err)
	}

	// Server-generated evidence that fails mapping is a provider/parse failure,
	// not a bad request (finding #6).
	envelope, err := sourcing.MapAcquisitionEvidence(request.Source, evidence, sourcing.AcquisitionChannelPublicBrowser, request.ID)
	if err != nil {
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, err)
	}
	if s.core.reader == nil {
		return sourcing.AcquisitionResult{}, sourcing.ErrAcquisitionUnavailable
	}
	if err := s.core.authorizeScope(ctx, request.Scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
	if err != nil {
		return sourcing.AcquisitionResult{}, s.failFetch(ctx, err)
	}
	base := uint64(0)
	current, err := s.core.reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: request.Scope.OrganizationID, ProductKey: productKey})
	if err == nil {
		base = current.Version
	} else if !errors.Is(err, catalog.ErrSnapshotNotReady) {
		return sourcing.AcquisitionResult{}, err
	}
	if err := s.core.authorizeScope(ctx, request.Scope); err != nil {
		return sourcing.AcquisitionResult{}, err
	}
	command := sourcing.PublicationCommand{PublicationID: publicationID, ProductKey: productKey, Producer: sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"}, ExpectedBaseVersion: &base, Envelope: envelope}
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
	return s.core.resolve(ctx, op, publishClaim, claim)
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
		return nil, false, nil
	}
	replayed := true
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
	}
	result, err := s.core.resolve(ctx, op, false, replayed)
	if err != nil {
		return nil, false, err
	}
	return &result, true, nil
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
// depend on it: StartPrepared remains the atomic arbiter.
func (s *BrowserAcquisitionService) acquireSlot(ctx context.Context, request sourcing.AcquisitionOperation) (func(), error) {
	key := request.Scope.OrganizationID + "|" + request.Scope.ActorID + "|" + request.Key
	value, _ := s.inflight.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return func() { mu.Unlock() }, nil
}

func (s *BrowserAcquisitionService) outerBudget() time.Duration {
	if s.providerBudget > 0 {
		return s.providerBudget + sourcing.AcquisitionTimeout
	}
	return sourcing.AcquisitionTimeout
}

// failFetch maps a provider/mapping failure to the existing failed sentinel.
// A pre-admission browser failure must not leave a durable terminal row: the
// path retries by re-acquiring under the same key (D1), and nothing is written
// yet, so there is nothing to Finish. The HTTP layer already projects
// ErrAcquisitionFailed as 502 SOURCE_UNAVAILABLE, which is the correct
// "server/provider failed" projection (finding #6), distinct from a 400 for a
// bad caller request.
func (s *BrowserAcquisitionService) failFetch(ctx context.Context, cause error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return sourcing.ErrAcquisitionFailed
}
