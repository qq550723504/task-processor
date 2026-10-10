// Package dataserviceauth consumes current IAM and native role policy. It owns
// no grants, memberships, user status, or bearer refresh workflow.
package dataserviceauth

import (
	"context"
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/dataservice"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/workbenchcontext"
	"time"
)

type ExactReader interface {
	ReadExactServiceProjectAuthorization(context.Context, string, string, string, string) (zitadel.ExactServiceProjectAuthorization, error)
}
type ActiveUserReader interface {
	IsUserActive(context.Context, string, string) (bool, error)
}
type Policy interface {
	authz.StaticAuthorizer
	IsTenantAdmin(string, []string) bool
}
type Authorizer struct {
	exact   ExactReader
	users   ActiveUserReader
	token   func(context.Context) (string, error)
	project string
	policy  Policy
	status  workbenchcontext.OrganizationBusinessStatusChecker
}

func NewAuthorizer(exact ExactReader, users ActiveUserReader, token func(context.Context) (string, error), project string, policy Policy, status workbenchcontext.OrganizationBusinessStatusChecker) (*Authorizer, error) {
	if exact == nil || users == nil || token == nil || !authidentity.IsBoundedIdentifier(project) || policy == nil || status == nil {
		return nil, dataservice.ErrUnavailable
	}
	return &Authorizer{exact, users, token, project, policy, status}, nil
}
func (a *Authorizer) current(ctx context.Context, scope collection.Scope) ([]string, error) {
	if ctx == nil || scope.Validate() != nil {
		return nil, dataservice.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, err := a.token(ctx)
	if err != nil || token == "" {
		return nil, dataservice.ErrUnavailable
	}
	grant, err := a.exact.ReadExactServiceProjectAuthorization(ctx, token, scope.ActorID, a.project, scope.OrganizationID)
	if err != nil {
		return nil, dataservice.ErrUnavailable
	}
	if !grant.Found || grant.State != "STATE_ACTIVE" || grant.AuthorizationID != scope.MemberID {
		return nil, dataservice.ErrForbidden
	}
	active, err := a.users.IsUserActive(ctx, token, scope.ActorID)
	if err != nil {
		return nil, dataservice.ErrUnavailable
	}
	if !active {
		return nil, dataservice.ErrForbidden
	}
	suspended, err := a.status.IsOrganizationSuspended(ctx, scope.OrganizationID)
	if err != nil || ctx.Err() != nil {
		return nil, dataservice.ErrUnavailable
	}
	if suspended {
		return nil, dataservice.ErrForbidden
	}
	return grant.Roles, nil
}
func permissions(p string) ([]string, error) {
	switch p {
	case dataservice.PermissionManage:
		return []string{dataservice.PermissionManage}, nil
	case dataservice.PermissionMarket:
		return []string{dataservice.PermissionMarket, collection.PermissionManage}, nil
	case dataservice.PermissionAcquire:
		return []string{dataservice.PermissionManage, dataservice.PermissionMarket, collection.PermissionManage}, nil
	case dataservice.PermissionResult:
		return []string{dataservice.PermissionManage, collection.PermissionRead}, nil
	case collection.PermissionRead:
		return []string{collection.PermissionRead}, nil
	default:
		return nil, dataservice.ErrForbidden
	}
}
func (a *Authorizer) allowed(ctx context.Context, scope collection.Scope, roles []string, required []string) error {
	for _, p := range required {
		allowed, err := authz.AuthorizeOrganization(ctx, a.policy, scope.ActorID, scope.OrganizationID, roles, p)
		if err != nil {
			return dataservice.ErrUnavailable
		}
		if !allowed {
			return dataservice.ErrForbidden
		}
	}
	return nil
}
func (a *Authorizer) Check(ctx context.Context, scope collection.Scope, p string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	required, err := permissions(p)
	if err != nil {
		return err
	}
	roles, err := a.current(ctx, scope)
	if err != nil {
		return err
	}
	return a.allowed(ctx, scope, roles, required)
}
func (a *Authorizer) Resolve(ctx context.Context, p string) (collection.Scope, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	scope := collection.Scope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID, MemberID: identity.EffectiveMemberID}
	if !ok || identity.TenantID != scope.OrganizationID || !identity.TokenExpiresAt.After(time.Now()) || scope.Validate() != nil {
		return collection.Scope{}, dataservice.ErrForbidden
	}
	if err := a.Check(ctx, scope, p); err != nil {
		return collection.Scope{}, err
	}
	return scope, nil
}
func acquisitionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, dataservice.ErrForbidden) {
		return dataacquisition.ErrForbidden
	}
	return dataacquisition.ErrUnavailable
}
func (a *Authorizer) CheckExecution(ctx context.Context, p dataacquisition.Principal, funding orgresource.ResourceFunding) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	roles, err := a.current(ctx, p.Scope)
	if err != nil {
		return acquisitionError(err)
	}
	required := []string{dataservice.PermissionMarket, collection.PermissionManage}
	if p.CredentialID != "" {
		required = append(required, dataservice.PermissionManage)
	}
	if err = a.allowed(ctx, p.Scope, roles, required); err != nil {
		return acquisitionError(err)
	}
	switch funding {
	case orgresource.FundingEnterprise:
		if !a.policy.IsTenantAdmin(p.Scope.ActorID, roles) {
			return dataacquisition.ErrForbidden
		}
	case orgresource.FundingMember:
	default:
		return dataacquisition.ErrInvalid
	}
	return nil
}
func (a *Authorizer) CheckRead(ctx context.Context, p dataacquisition.Principal) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	roles, err := a.current(ctx, p.Scope)
	if err != nil {
		return acquisitionError(err)
	}
	required := []string{collection.PermissionRead}
	if p.CredentialID != "" {
		required = append(required, dataservice.PermissionManage)
	}
	return acquisitionError(a.allowed(ctx, p.Scope, roles, required))
}
func (a *Authorizer) Funding(ctx context.Context, scope collection.Scope) (orgresource.ResourceFunding, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	roles, err := a.current(ctx, scope)
	if err != nil {
		return "", err
	}
	if err = a.allowed(ctx, scope, roles, []string{dataservice.PermissionMarket, collection.PermissionManage}); err != nil {
		return "", err
	}
	if a.policy.IsTenantAdmin(scope.ActorID, roles) {
		return orgresource.FundingEnterprise, nil
	}
	return orgresource.FundingMember, nil
}

func (a *Authorizer) Specialist(ctx context.Context) (dataservice.Operator, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !identity.TokenExpiresAt.After(time.Now()) || !authidentity.IsBoundedIdentifier(identity.UserID) || !a.policy.Authorize(identity.UserID, identity.Roles, authz.PermissionListingKitPlatformAdm) {
		return dataservice.Operator{}, dataservice.ErrForbidden
	}
	token, err := a.token(ctx)
	if err != nil || token == "" {
		return dataservice.Operator{}, dataservice.ErrUnavailable
	}
	active, err := a.users.IsUserActive(ctx, token, identity.UserID)
	if err != nil {
		return dataservice.Operator{}, dataservice.ErrUnavailable
	}
	if !active {
		return dataservice.Operator{}, dataservice.ErrForbidden
	}
	return dataservice.Operator{ID: identity.UserID}, nil
}
func (a *Authorizer) CheckApplicant(ctx context.Context, scope collection.Scope) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	roles, err := a.current(ctx, scope)
	if err != nil {
		return err
	}
	return a.allowed(ctx, scope, roles, []string{collection.PermissionManage})
}

var _ dataservice.SpecialistAccess = (*Authorizer)(nil)

var _ dataservice.Access = (*Authorizer)(nil)
var _ dataacquisition.LiveAccess = (*Authorizer)(nil)
