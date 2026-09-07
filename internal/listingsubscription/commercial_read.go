package listingsubscription

import (
	"context"
	"errors"
	"regexp"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

const CommercialResponseMaxBytes = 64 * 1024

var (
	ErrCommercialForbidden   = errors.New("commercial read permission denied")
	ErrCommercialUnavailable = errors.New("commercial read unavailable")
	commercialID             = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type CommercialPlanOption struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Source       string  `json:"source"`
	Availability string  `json:"availability"`
	Price        *string `json:"price"`
	Currency     *string `json:"currency"`
}

type CommercialSubscription struct {
	PlanCode        string     `json:"plan_code"`
	PlanName        *string    `json:"plan_name"`
	Status          string     `json:"status"`
	EffectiveStatus string     `json:"effective_status"`
	StartsAt        *time.Time `json:"starts_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CommercialLimit struct {
	Metric    string  `json:"metric"`
	SourceKey string  `json:"source_key"`
	Unit      string  `json:"unit"`
	Kind      string  `json:"kind"`
	RawValue  string  `json:"raw_value"`
	Value     *string `json:"value"`
}

type CommercialEntitlement struct {
	ModuleCode              string            `json:"module_code"`
	Status                  string            `json:"status"`
	EffectiveStatus         string            `json:"effective_status"`
	StartsAt                *time.Time        `json:"starts_at"`
	ExpiresAt               *time.Time        `json:"expires_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
	LimitsScope             string            `json:"limits_scope"`
	Limits                  []CommercialLimit `json:"limits"`
	UninterpretedLimitCount int               `json:"uninterpreted_limit_count"`
}

type CommercialUsage struct {
	ModuleCode  string     `json:"module_code"`
	Metric      string     `json:"metric"`
	Source      string     `json:"source"`
	Unit        string     `json:"unit"`
	PeriodKey   string     `json:"period_key"`
	WindowStart *time.Time `json:"window_start"`
	WindowEnd   *time.Time `json:"window_end"`
	State       string     `json:"state"`
	Committed   *string    `json:"committed"`
	Reserved    *string    `json:"reserved"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type UnsupportedCommercialValue struct {
	State string  `json:"state"`
	Value *string `json:"value"`
}

type CommercialOverview struct {
	OrganizationID  string                     `json:"organization_id"`
	ObservedAt      time.Time                  `json:"observed_at"`
	Plans           []CommercialPlanOption     `json:"plans"`
	Subscription    *CommercialSubscription    `json:"subscription"`
	Entitlements    []CommercialEntitlement    `json:"entitlements"`
	Usage           []CommercialUsage          `json:"usage"`
	ResourceBalance UnsupportedCommercialValue `json:"resource_balance"`
	CashBalance     UnsupportedCommercialValue `json:"cash_balance"`
}

// CommercialReadRepository reads the current owner, without constructing the
// legacy catalog-sync service, repairing grants or consulting a tenant bridge.
type CommercialReadRepository interface {
	ReadCommercialOverview(context.Context, string, time.Time) (*CommercialOverview, error)
}

type CommercialAuthorizer interface {
	Authorize(string, []string, string) bool
}

type CommercialReadService struct {
	repository CommercialReadRepository
	authorizer CommercialAuthorizer
}

func NewCommercialReadService(repository CommercialReadRepository, authorizer CommercialAuthorizer) *CommercialReadService {
	return &CommercialReadService{repository: repository, authorizer: authorizer}
}

func (s *CommercialReadService) Read(ctx context.Context) (*CommercialOverview, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !commercialID.MatchString(identity.EffectiveOrganizationID) || identity.TenantID != identity.EffectiveOrganizationID || identity.UserID == "" || !time.Now().Before(identity.TokenExpiresAt) {
		return nil, ErrCommercialForbidden
	}
	if s == nil || s.authorizer == nil || !s.authorizer.Authorize(identity.UserID, identity.Roles, authz.PermissionListingKitAdminRead) {
		return nil, ErrCommercialForbidden
	}
	if s.repository == nil {
		return nil, ErrCommercialUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := s.repository.ReadCommercialOverview(ctx, identity.EffectiveOrganizationID, time.Now().UTC())
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return nil, ErrCommercialForbidden
	}
	if err != nil || result == nil || result.OrganizationID != identity.EffectiveOrganizationID {
		return nil, ErrCommercialUnavailable
	}
	return result, nil
}
