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
			validator := listingAdminStoreAccessValidator{repo: storeAccessRepositoryStub{store: tt.store}}

			_, err := validator.ValidateStoreAccess(context.Background(), 101, 202, tt.platform)

			require.Equal(t, tt.wantCode, listingkit.StoreAccessErrorCode(err))
		})
	}
}

type storeAccessRepositoryStub struct {
	listingadmin.StoreRepository
	store *listingadmin.Store
}

func (s storeAccessRepositoryStub) GetStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	return s.store, nil
}
