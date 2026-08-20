package sourceaccount

import (
	"context"
	"time"
)

const (
	PlatformAlibaba1688       = "1688"
	StatusEnabled       int16 = 0
	StatusDisabled      int16 = 1
)

type AccessMode string

const (
	AccessModePublic          AccessMode = "public"
	AccessModeAccountAssisted AccessMode = "account_assisted"
)

// SourceAccount contains only non-secret account metadata and opaque runtime
// references. Secret material is resolved outside this domain.
type SourceAccount struct {
	ID             int64
	TenantID       int64
	Platform       string
	Label          string
	ProfileRef     string
	ProxyRef       string
	LoginURL       string
	Status         int16
	Deleted        int16
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repository interface {
	Get(context.Context, int64, int64) (*SourceAccount, error)
}

type Access struct {
	ID       int64
	TenantID int64
	Platform string
	Enabled  bool
}

type AccessValidator interface {
	ValidateSourceAccountAccess(context.Context, int64, int64) (Access, error)
}

func SelectAccessMode(accountID int64) (AccessMode, error) {
	if accountID < 0 {
		return "", NewUnavailableError("")
	}
	if accountID == 0 {
		return AccessModePublic, nil
	}
	return AccessModeAccountAssisted, nil
}
