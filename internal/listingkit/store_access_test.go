package listingkit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreAccessErrorCodeHidesForeignStore(t *testing.T) {
	err := NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")

	require.Equal(t, StoreAccessUnavailable, StoreAccessErrorCode(err))
	require.Equal(t, "listingkit_store_unavailable", StoreAccessErrorCode(err))
}

func TestStoreAccessErrorCodeRetainsDisabledStoreAction(t *testing.T) {
	err := NewStoreAccessError(StoreAccessDisabled, "store is disabled")

	require.Equal(t, "listingkit_store_disabled", StoreAccessErrorCode(err))
}

func TestServiceRetainsConfiguredStoreAccessValidator(t *testing.T) {
	validator := &storeAccessValidatorStub{}
	concrete := newServiceWithConfig(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.StoreAccessValidator = validator
		}),
	))

	require.Same(t, validator, resolveSheinStoreAccessValidator(concrete))
}

type storeAccessValidatorStub struct{}

func (*storeAccessValidatorStub) ValidateStoreAccess(context.Context, int64, int64, string) (StoreAccess, error) {
	return StoreAccess{}, nil
}
