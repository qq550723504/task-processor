package alibaba1688

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"task-processor/internal/sourceaccount"
)

const (
	AccountProfileUnavailable = sourceaccount.SourceAccountUnavailable
	AccountProfileDisabled    = sourceaccount.SourceAccountDisabled
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
	repository     sourceaccount.Repository
	profileRootDir string
}

// NewAccountProfileResolver builds a resolver backed by the dedicated source-account repository.
func NewAccountProfileResolver(repository sourceaccount.Repository, profileRootDir string) AccountProfileResolver {
	return &repositoryAccountProfileResolver{
		repository:     repository,
		profileRootDir: strings.TrimSpace(profileRootDir),
	}
}

func (r *repositoryAccountProfileResolver) ResolveAlibaba1688Account(ctx context.Context, tenantID, accountID int64) (AccountProfile, error) {
	if tenantID <= 0 || accountID <= 0 || strings.TrimSpace(r.profileRootDir) == "" || r.repository == nil {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	account, err := r.repository.Get(ctx, tenantID, accountID)
	if err != nil {
		if sourceaccount.ErrorCode(err) == sourceaccount.SourceAccountDisabled {
			return AccountProfile{}, newAccountProfileError(AccountProfileDisabled, "source account is disabled")
		}
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	if account == nil || account.ID != accountID || account.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(account.Platform), sourceaccount.PlatformAlibaba1688) {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	if account.Deleted != 0 {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	if account.Status != sourceaccount.StatusEnabled {
		return AccountProfile{}, newAccountProfileError(AccountProfileDisabled, "source account is disabled")
	}
	if strings.TrimSpace(account.ProfileRef) == "" {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	return AccountProfile{
		ID:         account.ID,
		TenantID:   account.TenantID,
		Label:      strings.TrimSpace(account.Label),
		ProfileDir: filepath.Join(r.profileRootDir, formatAccountPath(account.TenantID), formatAccountPath(account.ID)),
		LoginURL:   strings.TrimSpace(account.LoginURL),
	}, nil
}

func formatAccountPath(id int64) string {
	return strconv.FormatInt(id, 10)
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
	if errors.As(err, &profileErr) && profileErr != nil {
		return profileErr.code
	}
	return sourceAccessErrorCode(err)
}
