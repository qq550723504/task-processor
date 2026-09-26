package productsourcing

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type acquisitionAuthFunc func(context.Context) (sourcing.PublicationScope, error)

func (f acquisitionAuthFunc) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	return f(ctx)
}

type acquisitionStoreSpy struct {
	op   sourcing.AcquisitionOperation
	keys []string
}

func (s *acquisitionStoreSpy) Start(ctx context.Context, op sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	s.keys = append(s.keys, op.Fingerprint)
	return s.op, false, nil
}
func (s *acquisitionStoreSpy) ByKey(ctx context.Context, scope sourcing.PublicationScope, key string) (sourcing.AcquisitionOperation, error) {
	s.keys = append(s.keys, key)
	return s.op, nil
}
func (s *acquisitionStoreSpy) ByID(ctx context.Context, scope sourcing.PublicationScope, id string) (sourcing.AcquisitionOperation, error) {
	return s.op, nil
}
func (s *acquisitionStoreSpy) Prepare(context.Context, sourcing.AcquisitionOperation, sourcing.PublicationCommand) (sourcing.AcquisitionOperation, error) {
	panic("recovery must not prepare")
}
func (s *acquisitionStoreSpy) Claim(context.Context, sourcing.AcquisitionOperation) (sourcing.AcquisitionOperation, bool, error) {
	panic("verify must not claim")
}
func (s *acquisitionStoreSpy) Finish(context.Context, sourcing.AcquisitionOperation, string, string) error {
	return nil
}

type acquisitionPublisherSpy struct {
	verifies int
	command  sourcing.PublicationCommand
}

type acquisitionCatalogReaderSpy struct {
	identity  catalog.SnapshotIdentity
	version   uint64
	published catalog.PublishedSnapshot
	err       error
}

func (s *acquisitionCatalogReaderSpy) GetCurrentSnapshot(context.Context, catalog.SnapshotIdentity) (catalog.PublishedSnapshot, error) {
	return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
}
func (s *acquisitionCatalogReaderSpy) GetSnapshot(_ context.Context, identity catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.identity, s.version = identity, version
	return s.published, s.err
}

type acquisitionPublishedPublisherSpy struct {
	persisted sourcing.PersistedPublication
}

func (*acquisitionPublishedPublisherSpy) Publish(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	return sourcing.PublicationReceipt{}, sourcing.ErrAcquisitionUnavailable
}
func (*acquisitionPublishedPublisherSpy) Verify(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	return sourcing.PublicationReceipt{}, sourcing.ErrAcquisitionUnavailable
}
func (s *acquisitionPublishedPublisherSpy) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	return s.persisted, nil
}

func (s *acquisitionPublisherSpy) Publish(context.Context, sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	panic("unknown outcome must not Publish again")
}
func (s *acquisitionPublisherSpy) Verify(ctx context.Context, command sourcing.PublicationCommand) (sourcing.PublicationReceipt, error) {
	s.verifies++
	s.command = command
	return sourcing.PublicationReceipt{}, sourcing.ErrSourcePublicationNotFound
}
func (s *acquisitionPublisherSpy) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	panic("no publication exists")
}

func TestAcquisitionVerifyAuthorizesBeforeAnyOperationRead(t *testing.T) {
	store := &acquisitionStoreSpy{}
	service, err := NewAcquisitionService(store, nil, &acquisitionPublisherSpy{}, nil, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) {
		return sourcing.PublicationScope{}, sourcing.ErrPublicationForbidden
	}))
	require.NoError(t, err)
	_, err = service.Verify(context.Background(), uuid.NewString(), "123")
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
	require.Empty(t, store.keys)
}

func TestAcquisitionUnknownUsesFrozenCommandAcrossServiceRebuild(t *testing.T) {
	scope := sourcing.PublicationScope{OrganizationID: "effective-b", ActorID: "actor"}
	key := uuid.NewString()
	source, err := sourcing.Canonical1688Source("123")
	require.NoError(t, err)
	// The operation is the output of a prior acquiring/prepared/claim chain.
	// Its original payload is deliberately unrelated to a new provider response.
	command := sourcing.PublicationCommand{PublicationID: "original-publication", ProductKey: "crawler:1688:123", Envelope: sourcing.SourceEnvelope{ProductCandidate: sourcing.ProductCandidate{Title: "original captured title"}}}
	op := acquisitionOperation(scope, key, source)
	op.State, op.Command = sourcing.AcquisitionPublishing, &command
	store := &acquisitionStoreSpy{op: op}
	publisher := &acquisitionPublisherSpy{}
	for range 2 {
		service, err := NewAcquisitionService(store, nil, publisher, nil, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) { return scope, nil }))
		require.NoError(t, err)
		_, err = service.Verify(context.Background(), key, "https://detail.1688.com/offer/123.html?tracking=new")
		require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
		require.Equal(t, command, publisher.command)
	}
	require.Equal(t, 2, publisher.verifies)
	service, _ := NewAcquisitionService(store, nil, publisher, nil, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) { return scope, nil }))
	_, err = service.Verify(context.Background(), key, "124")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.Equal(t, 2, publisher.verifies)
}

func TestReadPublishedUsesOnlyTheOperationReceiptCatalogVersion(t *testing.T) {
	scope := sourcing.PublicationScope{OrganizationID: "organization-a", ActorID: "actor-a"}
	operationID := uuid.NewString()
	command := sourcing.PublicationCommand{PublicationID: "source-run:acquisition:" + operationID, ProductKey: "crawler:1688:981645030344"}
	op := sourcing.AcquisitionOperation{ID: operationID, Scope: scope, State: sourcing.AcquisitionPublished, Command: &command}
	receipt := sourcing.PublicationReceipt{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, PublicationID: command.PublicationID, CatalogPublicationID: command.PublicationID, ProductKey: command.ProductKey, CatalogVersion: 7}
	reader := &acquisitionCatalogReaderSpy{published: catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: command.ProductKey}, PublicationID: command.PublicationID, Version: 7, Snapshot: catalog.ProductSnapshot{Title: "Catalog fact"}}}
	service, err := NewAcquisitionService(&acquisitionStoreSpy{op: op}, nil, &acquisitionPublishedPublisherSpy{persisted: sourcing.PersistedPublication{Receipt: receipt}}, reader, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) { return scope, nil }))
	require.NoError(t, err)
	result, err := service.ReadPublished(context.Background(), operationID)
	require.NoError(t, err)
	require.Equal(t, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: command.ProductKey}, reader.identity)
	require.Equal(t, uint64(7), reader.version)
	require.Equal(t, "Catalog fact", result.Snapshot.Snapshot.Title)
}

func TestReadPublishedDoesNotExposeAnotherActorOrAnUnpublishedOperation(t *testing.T) {
	scope := sourcing.PublicationScope{OrganizationID: "organization-a", ActorID: "actor-a"}
	operationID := uuid.NewString()
	command := sourcing.PublicationCommand{PublicationID: "source-run:acquisition:" + operationID, ProductKey: "crawler:1688:981645030344"}
	reader := &acquisitionCatalogReaderSpy{}
	for _, tc := range []struct {
		name, actor, state string
		want               error
	}{
		{name: "other_actor", actor: "actor-b", state: sourcing.AcquisitionPublished, want: sourcing.ErrAcquisitionUnavailable},
		{name: "not_published", actor: scope.ActorID, state: sourcing.AcquisitionPrepared, want: sourcing.ErrAcquisitionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := sourcing.AcquisitionOperation{ID: operationID, Scope: scope, State: tc.state, Command: &command}
			service, err := NewAcquisitionService(&acquisitionStoreSpy{op: op}, nil, &acquisitionPublishedPublisherSpy{}, reader, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) {
				return sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: tc.actor}, nil
			}))
			require.NoError(t, err)
			_, err = service.ReadPublished(context.Background(), operationID)
			require.ErrorIs(t, err, tc.want)
		})
	}
	require.Empty(t, reader.identity)

	reader.err = catalog.ErrSnapshotNotReady
	op := sourcing.AcquisitionOperation{ID: operationID, Scope: scope, State: sourcing.AcquisitionPublished, Command: &command}
	receipt := sourcing.PublicationReceipt{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, PublicationID: command.PublicationID, CatalogPublicationID: command.PublicationID, ProductKey: command.ProductKey, CatalogVersion: 1}
	service, err := NewAcquisitionService(&acquisitionStoreSpy{op: op}, nil, &acquisitionPublishedPublisherSpy{persisted: sourcing.PersistedPublication{Receipt: receipt}}, reader, acquisitionAuthFunc(func(context.Context) (sourcing.PublicationScope, error) { return scope, nil }))
	require.NoError(t, err)
	_, err = service.ReadPublished(context.Background(), operationID)
	require.True(t, errors.Is(err, sourcing.ErrAcquisitionUnknown))
}
