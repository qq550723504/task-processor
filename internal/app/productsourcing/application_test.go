package productsourcing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"task-processor/internal/authz"
	"task-processor/internal/product/sourcing"
)

type liveRolesFunc func(context.Context, string, string) ([]string, error)

func (f liveRolesFunc) ResolveLiveRoles(ctx context.Context, organizationID, actorID string) ([]string, error) {
	return f(ctx, organizationID, actorID)
}

func TestNewInternalProducerRequiresPostgresAndLiveAuthorization(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	_, err = NewInternalProducer(nil, liveRolesFunc(func(context.Context, string, string) ([]string, error) { return nil, nil }), permissions)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationUnavailable)

	var nilDB *gorm.DB
	_, err = NewInternalProducer(nilDB, nil, permissions)
	require.Error(t, err)
	require.False(t, errors.Is(err, context.Canceled))
}
