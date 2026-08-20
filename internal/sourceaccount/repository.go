package sourceaccount

import "context"

// Repository and AccessValidator are intentionally separate from listingadmin
// store contracts so target marketplace stores cannot satisfy source access.
type RepositoryWithAccess interface {
	Repository
	AccessValidator
}

func ValidateAccess(ctx context.Context, repository Repository, tenantID, accountID int64) (Access, error) {
	if repository == nil || tenantID <= 0 || accountID <= 0 {
		return Access{}, NewUnavailableError("")
	}
	account, err := repository.Get(ctx, tenantID, accountID)
	if err != nil {
		return Access{}, err
	}
	if account == nil || account.ID != accountID || account.TenantID != tenantID || account.Platform != PlatformAlibaba1688 {
		return Access{}, NewUnavailableError("")
	}
	if account.Status != StatusEnabled || account.Deleted != 0 {
		return Access{}, NewDisabledError()
	}
	return Access{ID: account.ID, TenantID: account.TenantID, Platform: account.Platform, Enabled: true}, nil
}
