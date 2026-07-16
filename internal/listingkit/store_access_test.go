package listingkit

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/tenantbridge"
)

func TestMain(m *testing.M) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(storeAccessLegacyTenantResolver{})
	code := m.Run()
	restore()
	os.Exit(code)
}

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

func enableTestStoreAccess(s *service) {
	s.sheinSharedDeps.storeAccessValidator = &storeAccessValidatorStub{}
}

type storeAccessValidatorStub struct{}

func (*storeAccessValidatorStub) ValidateStoreAccess(context.Context, int64, int64, string) (StoreAccess, error) {
	return StoreAccess{}, nil
}

type storeAccessLegacyTenantResolver struct{}

func (storeAccessLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	if strings.TrimSpace(tenantID) == "tenant-a" {
		return 101, true, nil
	}
	return 0, false, nil
}
