package productsourcing

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	verifyErr                  error
	afterVerify                func()
}

func (p *browserTestPublisher) Publish(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	p.publishes++
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
			_, op, persisted := browserApplicationFixture(t)
			op.State = state
			if state == sourcing.AcquisitionAcquiring {
				op.Command = nil
			}
			store := &browserReadOnlyStore{op: op}
			publisher := &browserTestPublisher{persisted: persisted}
			auth := &browserTestAuthorizer{scope: op.Scope}
			service, err := NewBrowserCaptureService(store, publisher, nil, auth)
			require.NoError(t, err)
			result, err := service.ByKey(context.Background(), op.Key)
			switch state {
			case sourcing.AcquisitionAcquiring:
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				require.Zero(t, publisher.verifies)
			case sourcing.AcquisitionFailed:
				require.ErrorIs(t, err, sourcing.ErrAcquisitionFailed)
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

func TestBrowserCaptureRecoveryAuthUnknownAndWrongAction(t *testing.T) {
	for _, mode := range []string{"revoked", "switched org", "revoked during verify", "unknown receipt", "wrong action", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			_, op, persisted := browserApplicationFixture(t)
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
			_, err = service.ByKey(ctx, op.Key)
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
