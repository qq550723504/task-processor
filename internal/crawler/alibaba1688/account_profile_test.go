package alibaba1688

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"task-processor/internal/sourceaccount"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const accountProfileTestRoot = `C:\task-processor-test\1688-profiles`

func TestAccountProfileResolverResolveAlibaba1688Account(t *testing.T) {
	baseAccount := sourceaccount.SourceAccount{
		ID:         3001,
		TenantID:   101,
		Label:      "  1688 sourcing account  ",
		Platform:   sourceaccount.PlatformAlibaba1688,
		ProfileRef: "profile-ref",
		ProxyRef:   "proxy-ref",
		LoginURL:   "  https://login.1688.example  ",
		Status:     sourceaccount.StatusEnabled,
	}

	tests := []struct {
		name         string
		tenantID     int64
		accountID    int64
		account      *sourceaccount.SourceAccount
		getErr       error
		wantCode     string
		wantGetCalls int
		wantProfile  AccountProfile
	}{
		{
			name:         "enabled same tenant account resolves safe runtime profile",
			tenantID:     101,
			accountID:    3001,
			account:      &baseAccount,
			wantGetCalls: 1,
			wantProfile: AccountProfile{
				ID:         3001,
				TenantID:   101,
				Label:      "1688 sourcing account",
				ProfileDir: filepath.Join(accountProfileTestRoot, "101", "3001"),
				LoginURL:   "https://login.1688.example",
			},
		},
		{
			name:         "foreign tenant is unavailable",
			tenantID:     101,
			accountID:    3001,
			getErr:       sourceaccount.NewUnavailableError("foreign tenant"),
			wantCode:     AccountProfileUnavailable,
			wantGetCalls: 1,
		},
		{
			name:         "disabled account reports disabled",
			tenantID:     101,
			accountID:    3001,
			getErr:       sourceaccount.NewDisabledError(),
			wantCode:     AccountProfileDisabled,
			wantGetCalls: 1,
		},
		{
			name:      "other platform is unavailable",
			tenantID:  101,
			accountID: 3001,
			account: &sourceaccount.SourceAccount{
				ID: 3001, TenantID: 101, Platform: "SHEIN", ProfileRef: "target-store-row",
				Status: sourceaccount.StatusEnabled,
			},
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
			repository := &accountProfileSourceRepository{account: tt.account, getErr: tt.getErr}
			resolver := NewAccountProfileResolver(repository, accountProfileTestRoot)

			profile, err := resolver.ResolveAlibaba1688Account(context.Background(), tt.tenantID, tt.accountID)

			assert.Equal(t, tt.wantGetCalls, repository.getCalls)
			if tt.wantCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, AccountProfileErrorCode(err))
				assert.NotContains(t, err.Error(), "profile-ref")
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
	repository := &accountProfileSourceRepository{}
	resolver := NewAccountProfileResolver(repository, "  ")

	_, err := resolver.ResolveAlibaba1688Account(context.Background(), 101, 3001)

	require.Error(t, err)
	assert.Equal(t, AccountProfileUnavailable, AccountProfileErrorCode(err))
	assert.Equal(t, 0, repository.getCalls)
}

type accountProfileSourceRepository struct {
	account  *sourceaccount.SourceAccount
	getErr   error
	getCalls int
}

func (r *accountProfileSourceRepository) Get(_ context.Context, _, _ int64) (*sourceaccount.SourceAccount, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.account, nil
}

var _ sourceaccount.Repository = (*accountProfileSourceRepository)(nil)
