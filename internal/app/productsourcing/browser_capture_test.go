package productsourcing

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type browserTestAuthorizer struct {
	scope  sourcing.PublicationScope
	denied bool
}

func (a *browserTestAuthorizer) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	if ctx.Err() != nil {
		return sourcing.PublicationScope{}, ctx.Err()
	}
	if a.denied {
		return sourcing.PublicationScope{}, sourcing.ErrPublicationForbidden
	}
	return a.scope, nil
}

type browserReadOnlyStore struct {
	op            sourcing.AcquisitionOperation
	writes, reads int
}

func (s *browserReadOnlyStore) Start(context.Context, sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.writes++
	return sourcing.AcquisitionOperation{}, false, sourcing.ErrAcquisitionUnavailable
}
func (s *browserReadOnlyStore) Prepare(context.Context, sourcing.AcquisitionOperation, sourcing.PublicationCommand) (sourcing.AcquisitionOperation, error) {
	s.writes++
	return sourcing.AcquisitionOperation{}, sourcing.ErrAcquisitionUnavailable
}
func (s *browserReadOnlyStore) Claim(context.Context, sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.writes++
	return sourcing.AcquisitionOperation{}, false, sourcing.ErrAcquisitionUnavailable
}
func (s *browserReadOnlyStore) Finish(context.Context, sourcing.AcquisitionOperation, string, string) error {
	s.writes++
	return sourcing.ErrAcquisitionUnavailable
}
func (s *browserReadOnlyStore) ByKey(_ context.Context, scope sourcing.PublicationScope, key string) (sourcing.AcquisitionOperation, error) {
	s.reads++
	if scope != s.op.Scope || key != s.op.Key {
		return sourcing.AcquisitionOperation{}, sourcing.ErrAcquisitionNotFound
	}
	return s.op, nil
}
func (s *browserReadOnlyStore) ByID(_ context.Context, scope sourcing.PublicationScope, id string) (sourcing.AcquisitionOperation, error) {
	s.reads++
	if scope != s.op.Scope || id != s.op.ID {
		return sourcing.AcquisitionOperation{}, sourcing.ErrAcquisitionNotFound
	}
	return s.op, nil
}

type browserTestPublisher struct {
	persisted                  sourcing.PersistedPublication
	publishes, verifies, reads int
	publishReceipt             sourcing.PublicationReceipt
	publishErr                 error
	verifyErr                  error
	afterVerify                func()
}

type browserCaptureAdmissionStore struct {
	op                 sourcing.AcquisitionOperation
	startPreparedCalls int
	startPreparedErr   error
	finishCalls        int
	finishState        string
	claim              bool
	claimCalls         int
}

func (s *browserCaptureAdmissionStore) Start(_ context.Context, requested sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.op = requested
	s.op.State = sourcing.AcquisitionAcquiring
	s.op.Fence = 1
	return s.op, true, nil
}
func (s *browserCaptureAdmissionStore) StartPrepared(_ context.Context, requested sourcing.AcquisitionOperation, command sourcing.PublicationCommand) (sourcing.AcquisitionOperation, bool, error) {
	s.startPreparedCalls++
	if s.startPreparedErr != nil {
		return sourcing.AcquisitionOperation{}, false, s.startPreparedErr
	}
	s.op = requested
	s.op.Command = &command
	s.op.State = sourcing.AcquisitionPrepared
	s.op.Fence = 1
	return s.op, true, nil
}
func (s *browserCaptureAdmissionStore) ByKey(context.Context, sourcing.PublicationScope, string) (sourcing.AcquisitionOperation, error) {
	return s.op, nil
}
func (s *browserCaptureAdmissionStore) ByID(context.Context, sourcing.PublicationScope, string) (sourcing.AcquisitionOperation, error) {
	return s.op, nil
}
func (s *browserCaptureAdmissionStore) Prepare(context.Context, sourcing.AcquisitionOperation, sourcing.PublicationCommand) (sourcing.AcquisitionOperation, error) {
	return s.op, nil
}
func (s *browserCaptureAdmissionStore) Claim(context.Context, sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.claimCalls++
	if s.claim {
		s.op.State = sourcing.AcquisitionPublishing
	}
	return s.op, s.claim, nil
}
func (s *browserCaptureAdmissionStore) Finish(ctx context.Context, _ sourcing.AcquisitionOperation, state, reason string) error {
	s.finishCalls++
	s.finishState = state
	s.op.State, s.op.FailureCode = state, reason
	return nil
}

type browserCaptureAdmissionReader struct{ err error }

func (r browserCaptureAdmissionReader) GetCurrentSnapshot(context.Context, catalog.SnapshotIdentity) (catalog.PublishedSnapshot, error) {
	if r.err != nil {
		return catalog.PublishedSnapshot{}, r.err
	}
	return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
}
func (browserCaptureAdmissionReader) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
}

type browserCaptureCancelReader struct{ started chan struct{} }

func (r browserCaptureCancelReader) GetCurrentSnapshot(ctx context.Context, _ catalog.SnapshotIdentity) (catalog.PublishedSnapshot, error) {
	close(r.started)
	<-ctx.Done()
	return catalog.PublishedSnapshot{}, ctx.Err()
}

func (browserCaptureCancelReader) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
}

func (p *browserTestPublisher) Publish(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	p.publishes++
	if p.publishErr != nil {
		return sourcing.PublicationReceipt{}, p.publishErr
	}
	if p.publishReceipt.PublicationID != "" {
		return p.publishReceipt, nil
	}
	return sourcing.PublicationReceipt{}, sourcing.ErrAcquisitionUnavailable
}
func (p *browserTestPublisher) Verify(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	p.verifies++
	if p.afterVerify != nil {
		p.afterVerify()
	}
	return p.persisted.Receipt, p.verifyErr
}
func (p *browserTestPublisher) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	p.reads++
	return p.persisted, nil
}

func browserApplicationFixture(t *testing.T) ([]byte, sourcing.AcquisitionOperation, sourcing.PersistedPublication) {
	t.Helper()
	body, err := os.ReadFile("../../product/sourcing/testdata/browser-capture-v1.json")
	require.NoError(t, err)
	capture, err := sourcing.ParseBrowserCapture(body)
	require.NoError(t, err)
	op := acquisitionOperation(sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}, "d0ca04d0-1d36-4fce-8305-754c39244d09", capture.Source)
	op.CaptureSHA256 = capture.PayloadSHA256
	op.Fingerprint, err = sourcing.BrowserAcquisitionFingerprint(op.Source, op.CaptureSHA256)
	require.NoError(t, err)
	envelope, err := sourcing.MapAcquisitionEvidence(op.Source, capture.Evidence, "browser_capture", op.ID)
	require.NoError(t, err)
	envelope.RawReference.Metadata["capture_sha256"] = capture.PayloadSHA256
	productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	base := uint64(0)
	op.Command = &sourcing.PublicationCommand{ProductKey: productKey, PublicationID: publicationID, Producer: sourcing.ProducerDescriptor{Kind: sourcing.BrowserAcquisitionProducerKind, Version: "v1"}, ExpectedBaseVersion: &base, Envelope: envelope}
	op.State = sourcing.AcquisitionPublishing
	op.Fence = 1
	receipt := sourcing.PublicationReceipt{OrganizationID: op.Scope.OrganizationID, ActorID: op.Scope.ActorID, PublicationID: publicationID, CatalogPublicationID: publicationID, ProductKey: productKey, CatalogVersion: 1, InputHash: "test-input-hash", Producer: op.Command.Producer, ExpectedBaseVersion: &base}
	return body, op, sourcing.PersistedPublication{Receipt: receipt, Envelope: envelope}
}

func TestBrowserCaptureReadOnlyRecoveryAllStates(t *testing.T) {
	for _, state := range []string{sourcing.AcquisitionAcquiring, sourcing.AcquisitionPrepared, sourcing.AcquisitionPublishing, sourcing.AcquisitionPublished, sourcing.AcquisitionFailed} {
		t.Run(state, func(t *testing.T) {
			body, op, persisted := browserApplicationFixture(t)
			op.State = state
			if state == sourcing.AcquisitionAcquiring {
				op.Command = nil
			}
			store := &browserReadOnlyStore{op: op}
			publisher := &browserTestPublisher{persisted: persisted}
			auth := &browserTestAuthorizer{scope: op.Scope}
			service, err := NewBrowserCaptureService(store, publisher, nil, auth)
			require.NoError(t, err)
			result, err := service.Verify(context.Background(), op.Key, body)
			switch state {
			case sourcing.AcquisitionAcquiring:
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				require.Zero(t, publisher.verifies)
			case sourcing.AcquisitionFailed:
				require.NoError(t, err)
				require.Equal(t, sourcing.AcquisitionFailed, result.Operation.State)
				require.Zero(t, publisher.verifies)
			default:
				require.NoError(t, err)
				require.NotNil(t, result.Publication)
				require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
				require.Equal(t, 1, publisher.verifies)
				require.Equal(t, 1, publisher.reads)
			}
			require.Zero(t, store.writes)
			require.Zero(t, publisher.publishes)
			require.Equal(t, state, store.op.State, "projection must not mutate staging")
		})
	}
}

func TestBrowserCaptureVerifyAndReadNeverWrite(t *testing.T) {
	body, op, persisted := browserApplicationFixture(t)
	store := &browserReadOnlyStore{op: op}
	publisher := &browserTestPublisher{persisted: persisted}
	auth := &browserTestAuthorizer{scope: op.Scope}
	service, err := NewBrowserCaptureService(store, publisher, nil, auth)
	require.NoError(t, err)
	_, err = service.Verify(context.Background(), op.Key, body)
	require.NoError(t, err)
	_, err = service.Read(context.Background(), op.ID)
	require.NoError(t, err)
	require.Zero(t, store.writes)
	require.Zero(t, publisher.publishes)
	require.Equal(t, 2, publisher.verifies)
	changed := strings.Replace(string(body), "12.34000001", "12.34000002", 1)
	_, err = service.Verify(context.Background(), op.Key, []byte(changed))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.Equal(t, 2, publisher.verifies)
	require.Zero(t, store.writes)
}

func TestBrowserCapturePreflightFailureDoesNotAdmitOperation(t *testing.T) {
	body, _, persisted := browserApplicationFixture(t)
	for _, tc := range []struct {
		name      string
		readerErr error
		startErr  error
	}{
		{name: "snapshot read", readerErr: errors.New("snapshot unavailable")},
		{name: "atomic admission", startErr: errors.New("admission unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &browserCaptureAdmissionStore{startPreparedErr: tc.startErr}
			publisher := &browserTestPublisher{persisted: persisted}
			auth := &browserTestAuthorizer{scope: sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}}
			service, err := NewBrowserCaptureService(store, publisher, browserCaptureAdmissionReader{err: tc.readerErr}, auth)
			require.NoError(t, err)
			_, err = service.Capture(context.Background(), "d0ca04d0-1d36-4fce-8305-754c39244d09", body)
			require.Error(t, err)
			require.Zero(t, store.finishCalls)
			if tc.startErr != nil {
				require.Equal(t, 1, store.startPreparedCalls)
			} else {
				require.Zero(t, store.startPreparedCalls)
			}
		})
	}
}

func TestBrowserCaptureUsesAtomicPreparedAdmission(t *testing.T) {
	body, op, persisted := browserApplicationFixture(t)
	store := &browserCaptureAdmissionStore{claim: true}
	publisher := &browserTestPublisher{persisted: persisted, publishReceipt: persisted.Receipt}
	service, err := NewBrowserCaptureService(store, publisher, browserCaptureAdmissionReader{}, &browserTestAuthorizer{scope: op.Scope})
	require.NoError(t, err)

	result, err := service.Capture(context.Background(), op.Key, body)
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
	require.Equal(t, 1, store.startPreparedCalls)
	require.Equal(t, 1, store.finishCalls)
	require.Equal(t, sourcing.AcquisitionPublished, store.finishState)
	require.Equal(t, 1, publisher.publishes)
}

type browserCaptureFlippingAuthorizer struct {
	scope sourcing.PublicationScope
	calls int
}

func (a *browserCaptureFlippingAuthorizer) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	if err := ctx.Err(); err != nil {
		return sourcing.PublicationScope{}, err
	}
	a.calls++
	if a.calls > 1 {
		return sourcing.PublicationScope{}, sourcing.ErrPublicationForbidden
	}
	return a.scope, nil
}

func TestBrowserCaptureSecondAuthorizationFailureDoesNotAdmitOperation(t *testing.T) {
	body, _, persisted := browserApplicationFixture(t)
	store := &browserCaptureAdmissionStore{}
	authorizer := &browserCaptureFlippingAuthorizer{scope: sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}}
	service, err := NewBrowserCaptureService(store, &browserTestPublisher{persisted: persisted}, browserCaptureAdmissionReader{}, authorizer)
	require.NoError(t, err)

	_, err = service.Capture(context.Background(), "d0ca04d0-1d36-4fce-8305-754c39244d09", body)
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
	require.Zero(t, store.finishCalls)
	require.Zero(t, store.startPreparedCalls)
}

func TestBrowserCaptureAdmissionCancellationDoesNotAdmitOperation(t *testing.T) {
	body, _, persisted := browserApplicationFixture(t)
	store := &browserCaptureAdmissionStore{}
	reader := browserCaptureCancelReader{started: make(chan struct{})}
	authorizer := &browserTestAuthorizer{scope: sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}}
	service, err := NewBrowserCaptureService(store, &browserTestPublisher{persisted: persisted}, reader, authorizer)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, captureErr := service.Capture(ctx, "d0ca04d0-1d36-4fce-8305-754c39244d09", body)
		done <- captureErr
	}()
	<-reader.started
	cancel()

	require.ErrorIs(t, <-done, context.Canceled)
	require.Zero(t, store.finishCalls)
	require.Zero(t, store.startPreparedCalls)
}

func TestBrowserCaptureByKeyResumesDurablePrePublicationStates(t *testing.T) {
	_, prepared, persisted := browserApplicationFixture(t)
	for _, state := range []string{sourcing.AcquisitionPrepared, sourcing.AcquisitionPublishing} {
		t.Run(state, func(t *testing.T) {
			op := prepared
			op.State = state
			store := &browserCaptureAdmissionStore{op: op, claim: state == sourcing.AcquisitionPrepared}
			publisher := &browserTestPublisher{persisted: persisted, publishReceipt: persisted.Receipt}
			auth := &browserTestAuthorizer{scope: op.Scope}
			service, err := NewBrowserCaptureService(store, publisher, nil, auth)
			require.NoError(t, err)

			result, err := service.ByKey(context.Background(), op.Key)
			require.NoError(t, err)
			require.NotNil(t, result.Publication)
			require.Equal(t, sourcing.AcquisitionPublished, result.Operation.State)
			require.Equal(t, 1, publisher.publishes)
			if state == sourcing.AcquisitionPrepared {
				require.Equal(t, 1, store.claimCalls)
			}
		})
	}
}

func TestBrowserCaptureByKeyExposesDurableFailure(t *testing.T) {
	_, op, persisted := browserApplicationFixture(t)
	op.State = sourcing.AcquisitionFailed
	store := &browserCaptureAdmissionStore{op: op}
	publisher := &browserTestPublisher{persisted: persisted}
	service, err := NewBrowserCaptureService(store, publisher, nil, &browserTestAuthorizer{scope: op.Scope})
	require.NoError(t, err)

	result, err := service.ByKey(context.Background(), op.Key)
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionFailed, result.Operation.State)
	require.Nil(t, result.Publication)
	require.Zero(t, publisher.publishes)
}

func TestBrowserCaptureRecoveryAuthUnknownAndWrongAction(t *testing.T) {
	for _, mode := range []string{"revoked", "switched org", "revoked during verify", "unknown receipt", "wrong action", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			body, op, persisted := browserApplicationFixture(t)
			store := &browserReadOnlyStore{op: op}
			publisher := &browserTestPublisher{persisted: persisted}
			auth := &browserTestAuthorizer{scope: op.Scope}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "revoked":
				auth.denied = true
			case "switched org":
				auth.scope.OrganizationID = "other-org"
			case "revoked during verify":
				publisher.afterVerify = func() { auth.denied = true }
			case "unknown receipt":
				publisher.verifyErr = sourcing.ErrSourcePublicationNotFound
			case "wrong action":
				store.op = acquisitionOperation(op.Scope, op.Key, op.Source)
			case "canceled":
				cancel()
			}
			service, err := NewBrowserCaptureService(store, publisher, nil, auth)
			require.NoError(t, err)
			_, err = service.Verify(ctx, op.Key, body)
			require.Error(t, err)
			require.Zero(t, store.writes)
			require.Zero(t, publisher.publishes)
			if mode == "revoked" || mode == "canceled" {
				require.Zero(t, store.reads)
			}
			if mode == "revoked during verify" {
				require.Zero(t, publisher.reads)
			}
		})
	}
}
