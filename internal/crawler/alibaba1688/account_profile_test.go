package alibaba1688

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"task-processor/internal/listingadmin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const accountProfileTestRoot = `C:\task-processor-test\1688-profiles`

func TestAccountProfileResolverResolveAlibaba1688Account(t *testing.T) {
	secret := "fixture-password-must-not-leak"
	baseStore := listingadmin.Store{
		ID:       3001,
		TenantID: 101,
		Name:     "  1688 sourcing account  ",
		Platform: "1688",
		Status:   0,
		Proxy:    "  http://proxy.example:8080  ",
		LoginURL: "  https://login.1688.example  ",
		Password: secret,
	}

	tests := []struct {
		name         string
		tenantID     int64
		accountID    int64
		store        *listingadmin.Store
		getStoreErr  error
		wantCode     string
		wantGetCalls int
		wantProfile  AccountProfile
	}{
		{
			name:         "enabled same tenant 1688 account resolves safe runtime profile",
			tenantID:     101,
			accountID:    3001,
			store:        &baseStore,
			wantGetCalls: 1,
			wantProfile: AccountProfile{
				ID:          3001,
				TenantID:    101,
				Label:       "1688 sourcing account",
				ProfileDir:  filepath.Join(accountProfileTestRoot, "101", "3001"),
				ProxyServer: "http://proxy.example:8080",
				LoginURL:    "https://login.1688.example",
			},
		},
		{
			name:         "foreign tenant is unavailable",
			tenantID:     101,
			accountID:    3001,
			getStoreErr:  listingadmin.ErrStoreNotFound,
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 1,
		},
		{
			name:      "proxy userinfo is unavailable without exposing credentials",
			tenantID:  101,
			accountID: 3001,
			store: func() *listingadmin.Store {
				store := baseStore
				store.Proxy = "http://proxy-user:proxy-secret@proxy.example:8080"
				return &store
			}(),
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 1,
		},
		{
			name:      "disabled account reports disabled",
			tenantID:  101,
			accountID: 3001,
			store: func() *listingadmin.Store {
				store := baseStore
				store.Status = 1
				return &store
			}(),
			wantCode:     AccountProfileDisabled,
			wantGetCalls: 1,
		},
		{
			name:      "other platform is unavailable",
			tenantID:  101,
			accountID: 3001,
			store: func() *listingadmin.Store {
				store := baseStore
				store.Platform = "SHEIN"
				return &store
			}(),
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 1,
		},
		{
			name:         "non positive identifiers fail before repository access",
			tenantID:     0,
			accountID:    3001,
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 0,
		},
		{
			name:         "non positive account identifier fails before repository access",
			tenantID:     101,
			accountID:    0,
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &accountProfileStoreRepository{store: tt.store, getStoreErr: tt.getStoreErr}
			resolver := NewAccountProfileResolver(repository, accountProfileTestRoot)

			profile, err := resolver.ResolveAlibaba1688Account(context.Background(), tt.tenantID, tt.accountID)

			assert.Equal(t, tt.wantGetCalls, repository.getStoreCalls)
			if tt.wantCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, AccountProfileErrorCode(err))
				assert.NotContains(t, err.Error(), secret)
				assert.NotContains(t, err.Error(), "proxy-secret")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProfile, profile)
		})
	}
}

func TestAccountProfileContainsNoPasswordField(t *testing.T) {
	_, exists := reflect.TypeOf(AccountProfile{}).FieldByName("Password")
	assert.False(t, exists)
}

func TestAccountProfileResolverRejectsEmptyProfileRoot(t *testing.T) {
	repository := &accountProfileStoreRepository{}
	resolver := NewAccountProfileResolver(repository, "  ")

	_, err := resolver.ResolveAlibaba1688Account(context.Background(), 101, 3001)

	require.Error(t, err)
	assert.Equal(t, AccountProfileUnavailable, AccountProfileErrorCode(err))
	assert.Equal(t, 0, repository.getStoreCalls)
}

type accountProfileStoreRepository struct {
	store         *listingadmin.Store
	getStoreErr   error
	getStoreCalls int
}

func (r *accountProfileStoreRepository) GetStore(_ context.Context, _ int64, _ int64) (*listingadmin.Store, error) {
	r.getStoreCalls++
	if r.getStoreErr != nil {
		return nil, r.getStoreErr
	}
	return r.store, nil
}

func (r *accountProfileStoreRepository) ListStores(context.Context, listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) UpdateStoreID(context.Context, int64, string) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) DeleteStore(context.Context, int64, int64) error {
	return errors.New("not implemented")
}
func (r *accountProfileStoreRepository) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
func (r *accountProfileStoreRepository) PermanentlyDeleteStore(context.Context, int64, int64) error {
	return errors.New("not implemented")
}
func (r *accountProfileStoreRepository) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	return nil, errors.New("not implemented")
}
