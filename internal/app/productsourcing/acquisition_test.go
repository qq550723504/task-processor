package productsourcing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
