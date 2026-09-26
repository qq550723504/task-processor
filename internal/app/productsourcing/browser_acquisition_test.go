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
	p.delay = p.delay
	ev, err := p.evidence, p.err
	p.mu.Unlock()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		p.lastChild = true
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
	return admitted, true, nil
}
func (s *browserStore) Claim(_ context.Context, op sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	op.State = sourcing.AcquisitionPublishing
	return op, s.claimOK, nil
}
func (s *browserStore) Finish(_ context.Context, _ sourcing.AcquisitionOperation, state, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished++
	s.finishState = state
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
