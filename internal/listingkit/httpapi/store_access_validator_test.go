package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

func TestListingAdminStoreAccessValidatorRejectsForeignDisabledAndWrongPlatform(t *testing.T) {
	tests := []struct {
		name     string
		store    *listingadmin.Store
		platform string
		wantCode string
	}{
		{
			name:     "foreign tenant",
			store:    &listingadmin.Store{ID: 202, TenantID: 202, Platform: "SHEIN", Status: 0},
			platform: "SHEIN",
			wantCode: listingkit.StoreAccessUnavailable,
		},
		{
			name:     "disabled store",
			store:    &listingadmin.Store{ID: 202, TenantID: 101, Platform: "SHEIN", Status: 1},
			platform: "SHEIN",
			wantCode: listingkit.StoreAccessDisabled,
		},
		{
			name:     "wrong platform",
			store:    &listingadmin.Store{ID: 202, TenantID: 101, Platform: "1688", Status: 0},
			platform: "SHEIN",
			wantCode: listingkit.StoreAccessUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := listingAdminStoreAccessValidator{repo: &storeAccessRepositoryStub{store: tt.store}}

			_, err := validator.ValidateStoreAccess(context.Background(), 101, 202, tt.platform)

			require.Equal(t, tt.wantCode, listingkit.StoreAccessErrorCode(err))
		})
	}
}

type storeAccessRepositoryStub struct {
	listingadmin.StoreRepository
	store *listingadmin.Store
	ctx   context.Context
}

func (s *storeAccessRepositoryStub) GetStore(ctx context.Context, _ int64, _ int64) (*listingadmin.Store, error) {
	s.ctx = ctx
	return s.store, nil
}

func TestListingAdminStoreAccessValidatorPropagatesListingKitRoles(t *testing.T) {
	stub := &storeAccessRepositoryStub{
		store: &listingadmin.Store{ID: 202, TenantID: 101, Platform: "SHEIN", Status: 0},
	}
	ctx := listingkit.WithRequestRoles(context.Background(), []string{"listingkit_admin"})

	if _, err := (listingAdminStoreAccessValidator{repo: stub}).ValidateStoreAccess(ctx, 101, 202, "SHEIN"); err != nil {
		t.Fatalf("ValidateStoreAccess error = %v", err)
	}
	roles := listingadmin.RequestRolesFromContext(stub.ctx)
	if len(roles) != 1 || roles[0] != "listingkit_admin" {
		t.Fatalf("listingadmin roles = %v, want listingkit_admin", roles)
	}
}
