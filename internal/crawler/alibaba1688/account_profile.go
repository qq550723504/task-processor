package alibaba1688

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"task-processor/internal/listingadmin"
)

const (
	AccountProfileUnavailable = "alibaba1688_account_unavailable"
	AccountProfileDisabled    = "alibaba1688_account_disabled"
)

// AccountProfile is the safe browser runtime configuration for one 1688 login account.
type AccountProfile struct {
	ID          int64
	TenantID    int64
	Label       string
	ProfileDir  string
	ProxyServer string
	LoginURL    string
}

// AccountProfileResolver resolves a tenant-owned 1688 login account.
type AccountProfileResolver interface {
	ResolveAlibaba1688Account(context.Context, int64, int64) (AccountProfile, error)
}

type repositoryAccountProfileResolver struct {
	repository     listingadmin.StoreRepository
	profileRootDir string
}

// NewAccountProfileResolver builds a resolver backed by the tenant-scoped listing store repository.
func NewAccountProfileResolver(repository listingadmin.StoreRepository, profileRootDir string) AccountProfileResolver {
	return &repositoryAccountProfileResolver{
		repository:     repository,
		profileRootDir: strings.TrimSpace(profileRootDir),
	}
}

func (r *repositoryAccountProfileResolver) ResolveAlibaba1688Account(ctx context.Context, tenantID, accountID int64) (AccountProfile, error) {
	if tenantID <= 0 || accountID <= 0 || strings.TrimSpace(r.profileRootDir) == "" || r.repository == nil {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	store, err := r.repository.GetStore(ctx, tenantID, accountID)
	if err != nil || store == nil || store.ID != accountID || store.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(store.Platform), "1688") {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	if store.Status != 0 {
		return AccountProfile{}, newAccountProfileError(AccountProfileDisabled, "1688 account is disabled")
	}
	proxyServer := strings.TrimSpace(store.Proxy)
	if proxyServer != "" {
		proxyURL, err := url.Parse(proxyServer)
		if err != nil || proxyURL.User != nil {
			return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
		}
	}

	return AccountProfile{
		ID:          store.ID,
		TenantID:    store.TenantID,
		Label:       strings.TrimSpace(store.Name),
		ProfileDir:  filepath.Join(r.profileRootDir, strconv.FormatInt(store.TenantID, 10), strconv.FormatInt(store.ID, 10)),
		ProxyServer: proxyServer,
		LoginURL:    strings.TrimSpace(store.LoginURL),
	}, nil
}

type accountProfileError struct {
	code    string
	message string
}

func (e *accountProfileError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func newAccountProfileError(code, message string) error {
	return &accountProfileError{code: code, message: message}
}

// AccountProfileErrorCode returns a stable account-profile resolution error code.
func AccountProfileErrorCode(err error) string {
	var profileErr *accountProfileError
	if !errors.As(err, &profileErr) || profileErr == nil {
		return ""
	}
	return profileErr.code
}
