package productsourcing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

// ---- test doubles -------------------------------------------------------

type browserProviderSpy struct {
	mu        sync.Mutex
	calls     int
	evidence  sourcing.AcquisitionEvidence
	err       error
	delay     time.Duration
	lastChild bool // whether Acquire received a child (budgeted) context
}

func (p *browserProviderSpy) Acquire(ctx context.Context, _ sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	p.mu.Lock()
	p.calls++
	ev, err := p.evidence, p.err
	p.mu.Unlock()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		p.mu.Lock()
		p.lastChild = true
		p.mu.Unlock()
	}
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return sourcing.AcquisitionEvidence{}, ctx.Err()
		}
	}
	if err != nil {
		return sourcing.AcquisitionEvidence{}, err
	}
	return ev, nil
}

func (p *browserProviderSpy) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func browserEvidence() sourcing.AcquisitionEvidence {
	title, amount, currency, sku := "Browser bottle", "12.50", "CNY", "sku-1"
	return sourcing.AcquisitionEvidence{
		SchemaVersion: 1, OfferID: "981645030344", SourceURL: "https://detail.1688.com/offer/981645030344.html",
		Title: &title, ContentSHA256: repeated('d', 64), ParserVersion: "1688-browser-dom/v1",
		CapturedAt: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		Attributes: []sourcing.AcquisitionAttribute{{Name: "material", Value: "steel"}},
		Variants:   []sourcing.AcquisitionVariant{{SourceID: &sku, Price: &sourcing.AcquisitionPrice{Amount: amount, Currency: &currency}}},
		Images:     []sourcing.AcquisitionImage{{URL: "https://cbu01.alicdn.com/x.jpg", Role: "primary"}},
	}
}

func repeated(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

// browserStore is a PreparedAcquisitionOperationStore + capacity reader.
type browserStore struct {
	mu          sync.Mutex
	byKey       map[string]sourcing.AcquisitionOperation
	byKeyErr    error
	prepared    bool // StartPrepared admits
	capacity    bool
	capErr      error
	startPrep   int
	claims      int
	claimOK     bool
	finished    int
	finishState string
}

func newBrowserStore() *browserStore {
	return &browserStore{byKey: map[string]sourcing.AcquisitionOperation{}, capacity: true, claimOK: true}
}

func (s *browserStore) Start(context.Context, sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	panic("browser path must not use Start")
}
func (s *browserStore) ByKey(_ context.Context, scope sourcing.PublicationScope, key string) (sourcing.AcquisitionOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byKeyErr != nil {
		return sourcing.AcquisitionOperation{}, s.byKeyErr
	}
	op, ok := s.byKey[scope.OrganizationID+"|"+key]
	if !ok {
		return sourcing.AcquisitionOperation{}, sourcing.ErrAcquisitionNotFound
	}
	return op, nil
}
func (s *browserStore) ByID(context.Context, sourcing.PublicationScope, string) (sourcing.AcquisitionOperation, error) {
	return sourcing.AcquisitionOperation{}, sourcing.ErrAcquisitionNotFound
}
func (s *browserStore) Prepare(context.Context, sourcing.AcquisitionOperation, sourcing.PublicationCommand) (sourcing.AcquisitionOperation, error) {
	panic("browser path must not use Prepare")
}
func (s *browserStore) StartPrepared(_ context.Context, op sourcing.AcquisitionOperation, cmd sourcing.PublicationCommand) (sourcing.AcquisitionOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startPrep++
	if !s.prepared {
		return sourcing.AcquisitionOperation{}, false, errors.New("boom")
	}
	admitted := op
	admitted.State = sourcing.AcquisitionPrepared
	admitted.Command = &cmd
	// A real store persists the row here, so ByKey can find it. The double must
	// do the same or a same-key replay path can never be exercised.
	s.byKey[admitted.Scope.OrganizationID+"|"+admitted.Key] = admitted
	return admitted, true, nil
}
func (s *browserStore) Claim(_ context.Context, op sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	op.State = sourcing.AcquisitionPublishing
	return op, s.claimOK, nil
}
func (s *browserStore) Finish(_ context.Context, op sourcing.AcquisitionOperation, state, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished++
	s.finishState = state
	// Keep the durable row in step with its terminal state so a later ByKey
	// read observes what a real store would return.
	stored, ok := s.byKey[op.Scope.OrganizationID+"|"+op.Key]
	if ok {
		stored.State = state
		s.byKey[op.Scope.OrganizationID+"|"+op.Key] = stored
	}
	return nil
}
func (s *browserStore) CapacityAdmitted(context.Context, sourcing.PublicationScope) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capacity, s.capErr
}

// publisher that accepts the first publish and records it.
type recordingPublisher struct {
	mu        sync.Mutex
	publishes int
}

func (p *recordingPublisher) Publish(_ context.Context, cmd sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes++
	return sourcing.PublicationReceipt{PublicationID: cmd.PublicationID, CatalogPublicationID: cmd.PublicationID, ProductKey: cmd.ProductKey, CatalogVersion: 1, OrganizationID: "browser-org", ActorID: "browser-actor", InputHash: "h", Producer: cmd.Producer}, nil
}
func (p *recordingPublisher) Verify(ctx context.Context, cmd sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	return p.Publish(ctx, cmd)
}
func (p *recordingPublisher) Read(_ context.Context, id string) (sourcing.PersistedPublication, error) {
	return sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{PublicationID: id, CatalogPublicationID: id, ProductKey: "crawler:1688:981645030344", CatalogVersion: 1, OrganizationID: "browser-org", ActorID: "browser-actor", InputHash: "h"}}, nil
}

func testScope() sourcing.PublicationScope {
	return sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}
}

func newBrowserService(t *testing.T, store sourcing.AcquisitionOperationStore, provider sourcing.PublicAcquirer) *BrowserAcquisitionService {
	t.Helper()
	authorizer := browserAuthStub{scope: testScope()}
	reader := &acquisitionCatalogReaderSpy{}
	svc, err := NewBrowserAcquisitionService(store, provider, &recordingPublisher{}, reader, authorizer, 90*time.Second)
	require.NoError(t, err)
	return svc
}

type browserAuthStub struct{ scope sourcing.PublicationScope }

func (a browserAuthStub) Authorize(context.Context) (sourcing.PublicationScope, error) {
	return a.scope, nil
}

// ---- behavior tests -----------------------------------------------------

// A first-time POST with a fresh key must acquire, prepare, claim, and publish.
func TestBrowserAcquireFirstTimePublishes(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence()}
	svc := newBrowserService(t, store, provider)
	result, err := svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
	require.Equal(t, 1, provider.count())
	require.Equal(t, 1, store.startPrep)
	require.Equal(t, 1, store.claims)
	require.Equal(t, "crawler:1688:981645030344", result.Publication.Receipt.ProductKey)
}

// A first-time publish must transition the operation to published and must
// never write a terminal failed row for a successful run (D1: the browser path
// admits atomically, so a failure is not pre-recorded as a durable failed row).
func TestBrowserAcquireDoesNotLeaveFailedRow(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence()}
	svc := newBrowserService(t, store, provider)
	_, err := svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPublished, store.finishState, "successful run must finish as published, never failed")
}

// A response-loss retry (durable published op already present) must resolve
// WITHOUT invoking the provider again (finding #5).
func TestBrowserAcquireReplaysPublishedWithoutProvider(t *testing.T) {
	store := newBrowserStore()
	provider := &browserProviderSpy{evidence: browserEvidence()}
	key := "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	// Build a durable published op for this scope+key.
	replayOp := publishedOperationFor(t, testScope(), key, "981645030344", sourcing.AcquisitionPublished)
	store.byKey[testScope().OrganizationID+"|"+key] = replayOp
	svc := newBrowserService(t, store, provider)
	result, err := svc.Acquire(context.Background(), key, "981645030344")
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
	require.Equal(t, 0, provider.count(), "replay must not launch the browser")
}

// A same-key/different-offer request must conflict before any acquisition
// (finding #16).
func TestBrowserAcquireSameKeyDifferentOfferConflicts(t *testing.T) {
	store := newBrowserStore()
	provider := &browserProviderSpy{evidence: browserEvidence()}
	key := "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	store.byKey[testScope().OrganizationID+"|"+key] = publishedOperationFor(t, testScope(), key, "111111111111", sourcing.AcquisitionPublished)
	svc := newBrowserService(t, store, provider)
	_, err := svc.Acquire(context.Background(), key, "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.Equal(t, 0, provider.count(), "conflict must be detected before acquiring")
}

// A prepared operation (response lost after StartPrepared, before Claim) must
// be claimed and advanced, not left as OUTCOME_UNKNOWN forever (finding #20).
func TestBrowserAcquireClaimsPreparedOnReplay(t *testing.T) {
	store := newBrowserStore()
	provider := &browserProviderSpy{evidence: browserEvidence()}
	key := "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	prepared := publishedOperationFor(t, testScope(), key, "981645030344", sourcing.AcquisitionPublished)
	prepared.State = sourcing.AcquisitionPrepared
	store.byKey[testScope().OrganizationID+"|"+key] = prepared
	svc := newBrowserService(t, store, provider)
	result, err := svc.Acquire(context.Background(), key, "981645030344")
	require.NoError(t, err)
	require.Equal(t, 1, store.claims, "prepared must be claimed before resolve")
	require.Equal(t, 0, provider.count(), "prepared replay must not launch the browser")
	require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
}

// A capped organization must be rejected by a bounded preflight BEFORE the
// provider runs (finding #15/#19).
func TestBrowserAcquireCapacityPreflightSkipsProvider(t *testing.T) {
	store := newBrowserStore()
	store.capacity = false
	provider := &browserProviderSpy{evidence: browserEvidence()}
	svc := newBrowserService(t, store, provider)
	_, err := svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionCapacity)
	require.Equal(t, 0, provider.count(), "capped organization must not launch the browser")
}

// Evidence that fails mapping is a provider failure projected as
// ErrAcquisitionFailed (the HTTP layer renders 502 SOURCE_UNAVAILABLE), not a
// 400 for the caller (finding #6).
func TestBrowserAcquireEvidenceMappingFailureIsSourceFailure(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	bad := browserEvidence()
	bad.ContentSHA256 = "not-a-digest" // mapper rejects this
	provider := &browserProviderSpy{evidence: bad}
	svc := newBrowserService(t, store, provider)
	_, err := svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionFailed)
	require.NotErrorIs(t, err, sourcing.ErrInvalidAcquisition)
	require.Equal(t, 0, store.startPrep, "mapping failure must not admit an operation")
}

// The provider receives a bounded child context; the outer budget still covers
// publication (finding #9). We assert the provider sees a deadline.
func TestBrowserProviderReceivesBoundedChildContext(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence()}
	svc := newBrowserService(t, store, provider)
	_, err := svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.NoError(t, err)
	require.True(t, provider.lastChild, "provider must run under a bounded child context")
}

// publishedOperationFor builds a durable operation for (scope,key) over the
// given offer, in the given state, with a command/fingerprint that resolve()
// and sameAcquisition() accept. It mirrors acquisitionOperation()'s identity.
func publishedOperationFor(t *testing.T, scope sourcing.PublicationScope, key, offerID string, state string) sourcing.AcquisitionOperation {
	t.Helper()
	source, err := sourcing.Canonical1688Source(offerID)
	if err != nil {
		t.Fatal(err)
	}
	op := acquisitionOperation(scope, key, source)
	op.State = state

	// Build a command whose envelope is accepted by MapAcquisitionEvidence and
	// whose publication identity is stable.
	ev := browserEvidence()
	ev.OfferID, ev.SourceURL = source.OfferID, source.URL
	envelope, err := sourcing.MapAcquisitionEvidence(source, ev, sourcing.AcquisitionChannelPublicBrowser, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
	if err != nil {
		t.Fatal(err)
	}
	base := uint64(0)
	cmd := sourcing.PublicationCommand{
		PublicationID: publicationID, ProductKey: productKey,
		Producer:            sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"},
		ExpectedBaseVersion: &base, Envelope: envelope,
	}
	op.Command = &cmd
	return op
}

// A provider-scoped deadline must surface as DEADLINE_EXCEEDED (504), not as a
// 502 source failure, even though the outer publication budget is still alive.
func TestBrowserProviderTimeoutIsDeadlineExceeded(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	// Provider blocks until its child context expires.
	provider := &browserProviderSpy{delay: 200 * time.Millisecond}
	svc, err := NewBrowserAcquisitionService(store, provider, &recordingPublisher{}, &acquisitionCatalogReaderSpy{}, browserAuthStub{scope: testScope()}, 30*time.Millisecond)
	require.NoError(t, err)
	_, err = svc.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 0, store.startPrep, "timeout must not admit an operation")
}

// The same-key admission map must not retain entries after use: a long-lived
// process would otherwise grow one entry per distinct (org, actor, key).
func TestBrowserAcquisitionInflightMapDrainsAfterUse(t *testing.T) {
	service := &BrowserAcquisitionService{}
	request := sourcing.AcquisitionOperation{
		Scope: sourcing.PublicationScope{OrganizationID: "org-1", ActorID: "actor-1"},
		Key:   "key-1",
	}
	for i := 0; i < 3; i++ {
		release, err := service.acquireSlot(context.Background(), request)
		require.NoError(t, err)
		release()
	}
	service.inflightMu.Lock()
	remaining := len(service.inflight)
	service.inflightMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected the admission map to drain, got %d entries", remaining)
	}
}

// Concurrent same-key callers must serialize, and the map must still drain
// once every holder has released.
func TestBrowserAcquisitionInflightSerializesAndDrains(t *testing.T) {
	service := &BrowserAcquisitionService{}
	request := sourcing.AcquisitionOperation{
		Scope: sourcing.PublicationScope{OrganizationID: "org-1", ActorID: "actor-1"},
		Key:   "key-1",
	}
	var concurrent, peak int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := service.acquireSlot(context.Background(), request)
			require.NoError(t, err)
			mu.Lock()
			concurrent++
			if concurrent > peak {
				peak = concurrent
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			concurrent--
			mu.Unlock()
			release()
		}()
	}
	wg.Wait()
	if peak != 1 {
		t.Fatalf("expected same-key callers to serialize, peak concurrency was %d", peak)
	}
	service.inflightMu.Lock()
	remaining := len(service.inflight)
	service.inflightMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected the admission map to drain, got %d entries", remaining)
	}
}

// A same-key waiter must replay the leader's operation instead of failing the
// capacity preflight. Scenario: the organization is at its last slot, so the
// leader's acquisition consumes it; a concurrent same-key POST that missed the
// initial ByKey must still receive the leader's terminal result rather than
// ACQUISITION_CAPACITY (Codex finding on admission coordination).
func TestBrowserAcquireSameKeyWaiterReplaysInsteadOfCapacityError(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence(), delay: 150 * time.Millisecond}
	service := newBrowserService(t, store, provider)

	type outcome struct {
		result sourcing.AcquisitionResult
		err    error
	}
	results := make(chan outcome, 2)
	acquire := func() {
		res, err := service.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "981645030344")
		results <- outcome{res, err}
	}

	go acquire()
	// Let the leader get far enough to be inside the provider call, then close
	// capacity so the organization is exactly at its last slot.
	require.Eventually(t, func() bool { return provider.count() == 1 }, 2*time.Second, 5*time.Millisecond)
	store.mu.Lock()
	store.capacity = false
	store.mu.Unlock()

	go acquire()

	first := <-results
	second := <-results

	// Exactly one provider call: the waiter did not launch a second browser.
	require.Equal(t, 1, provider.count(), "the same-key waiter must not re-acquire")

	// Neither caller may see a capacity error: one publishes, the other replays.
	for i, got := range []outcome{first, second} {
		require.NoError(t, got.err, "caller %d must not fail with a capacity error (err=%v replayed=%v)", i, got.err, got.result.Replayed)
		require.Equal(t, sourcing.AcquisitionPublished, got.result.Operation.State, "caller %d", i)
	}
	require.Equal(t, first.result.Operation.ID, second.result.Operation.ID, "both callers must observe the same operation")
}

// A cancelled same-key waiter must not stay blocked until the preceding browser
// call finishes; the wait has to observe the request context.
func TestBrowserAcquireSameKeyWaitHonorsCancellation(t *testing.T) {
	service := &BrowserAcquisitionService{}
	request := sourcing.AcquisitionOperation{
		Scope: sourcing.PublicationScope{OrganizationID: "org-1", ActorID: "actor-1"},
		Key:   "key-cancel",
	}
	// Hold the per-key lock.
	held, err := service.acquireSlot(context.Background(), request)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.acquireSlot(ctx, request)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled same-key waiter must not remain blocked")
	}
	held()
}

// A collector concurrency-limit rejection is retryable. Flattening it into a
// source failure would report 502 SOURCE_UNAVAILABLE and, on the generic
// service, persist the idempotency key as permanently failed.
func TestBrowserAcquireKeepsCollectorCapacityRetryable(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence(), err: sourcing.ErrAcquisitionCapacity}
	service := newBrowserService(t, store, provider)
	_, err := service.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4e", "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionCapacity)
	require.NotErrorIs(t, err, sourcing.ErrAcquisitionFailed)
	require.Equal(t, 0, store.startPrep, "a capacity rejection must not admit an operation")
}

// A collector outage or a rejected service credential is an availability
// failure, not a 1688 source failure, and must not be flattened into one.
func TestBrowserAcquireKeepsCollectorAvailabilityDistinct(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence(), err: sourcing.ErrAcquisitionUnavailable}
	service := newBrowserService(t, store, provider)
	_, err := service.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4f", "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
	require.NotErrorIs(t, err, sourcing.ErrAcquisitionFailed)
	require.Equal(t, 0, store.startPrep, "an unavailable collector must not admit an operation")
}

// A durable row that already exists for the key but has no command must be
// reported from that row, never by launching a browser that StartPrepared would
// only rediscover. This matters when the collector is enabled after the HTTP
// provider left an acquiring or failed row.
func TestBrowserAcquireReplaysPreCommandRowWithoutProviderWork(t *testing.T) {
	store := newBrowserStore()
	store.prepared = true
	provider := &browserProviderSpy{evidence: browserEvidence()}
	service := newBrowserService(t, store, provider)
	scope := testScope()
	// A terminal row left by another path for this same key.
	store.byKey[scope.OrganizationID+"|8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c50"] = sourcing.AcquisitionOperation{
		Scope: scope, Key: "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c50", State: sourcing.AcquisitionFailed,
	}
	_, err := service.Acquire(context.Background(), "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c50", "981645030344")
	require.Error(t, err)
	require.Equal(t, 0, provider.count(), "an existing pre-command row must not launch a browser")
}
