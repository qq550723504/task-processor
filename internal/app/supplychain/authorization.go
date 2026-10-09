package supplychainapp

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
)

// The same current owner contract serves request and worker callers. Original
// member identity comes from a verified request or the durable scoped command;
// only live exact IAM grants and current role policy grant execution.
type OrganizationExecutionAuthorizer struct {
	Client             *zitadel.AuthorizationClient
	ServiceToken       func(context.Context) (string, error)
	ProjectID          string
	Permissions        *authz.ListingKitAuthorizer
	OrganizationStatus workbenchcontext.OrganizationBusinessStatusChecker
}

func (a OrganizationExecutionAuthorizer) current(ctx context.Context, scope collection.Scope) ([]string, error) {
	if ctx == nil || scope.Validate() != nil {
		return nil, collection.ErrForbidden
	}
	if ctx.Err() != nil || a.Client == nil || a.ServiceToken == nil || a.ProjectID == "" || a.Permissions == nil {
		return nil, collection.ErrUnavailable
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return nil, collection.ErrForbidden
	}
	if identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx); ok && (identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || !time.Now().Before(identity.TokenExpiresAt)) {
		return nil, collection.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, err := a.ServiceToken(ctx)
	if err != nil {
		return nil, collection.ErrUnavailable
	}
	grant, err := a.Client.ReadExactServiceProjectAuthorization(ctx, token, scope.ActorID, a.ProjectID, scope.OrganizationID)
	if err != nil {
		return nil, collection.ErrUnavailable
	}
	if !grant.Found || grant.State != "STATE_ACTIVE" || grant.AuthorizationID != scope.MemberID {
		return nil, collection.ErrForbidden
	}
	if a.OrganizationStatus != nil {
		suspended, err := a.OrganizationStatus.IsOrganizationSuspended(ctx, scope.OrganizationID)
		if err != nil {
			return nil, collection.ErrUnavailable
		}
		if suspended {
			return nil, collection.ErrForbidden
		}
	}
	if ctx.Err() != nil {
		return nil, collection.ErrUnavailable
	}
	return grant.Roles, nil
}
func (a OrganizationExecutionAuthorizer) AuthorizeExecution(ctx context.Context, scope collection.Scope, permission string) error {
	if permission != collection.PermissionRead && permission != collection.PermissionManage && permission != preparation.PermissionRead && permission != preparation.PermissionManage && permission != preparation.PermissionSubmit {
		return collection.ErrForbidden
	}
	roles, err := a.current(ctx, scope)
	if err != nil {
		return err
	}
	allowed, err := authz.AuthorizeOrganization(ctx, a.Permissions, scope.ActorID, scope.OrganizationID, roles, permission)
	if err != nil {
		return collection.ErrUnavailable
	}
	if !allowed {
		return collection.ErrForbidden
	}
	return nil
}

// Agent consumers must obtain their own live module grants. Supply manage
// never substitutes for Product Agent, source evidence or Review authority.
func (a OrganizationExecutionAuthorizer) ResolveAgentExecution(ctx context.Context, scope collection.Scope) ([]string, error) {
	roles, err := a.current(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, permission := range []string{collection.PermissionRead, preparation.PermissionRead, preparation.PermissionManage, authz.PermissionLocalAgentWrite, authz.PermissionWorkbenchAgentUse, authz.PermissionProductSourcingWrite} {
		allowed, err := authz.AuthorizeOrganization(ctx, a.Permissions, scope.ActorID, scope.OrganizationID, roles, permission)
		if err != nil {
			return nil, collection.ErrUnavailable
		}
		if !allowed {
			return nil, collection.ErrForbidden
		}
	}
	return append([]string(nil), roles...), nil
}
func (a OrganizationExecutionAuthorizer) AuthorizeProductExecution(ctx context.Context, subject storecenter.ProductExecutionSubject) (storecenter.ProductExecutionAuthorization, error) {
	scope := collection.Scope{OrganizationID: subject.OrganizationID, ActorID: subject.ActorID, MemberID: subject.MemberID}
	permissions := []string{preparation.PermissionRead, authz.PermissionWorkbenchStoreRead}
	switch subject.Purpose {
	case storecenter.ProductPurposeRules:
	case storecenter.ProductPurposePublish, storecenter.ProductPurposeImage:
		permissions = append(permissions, collection.PermissionRead, preparation.PermissionManage, preparation.PermissionSubmit)
	default:
		return storecenter.ProductExecutionAuthorization{}, storecenter.ErrNotFound
	}
	roles, err := a.current(ctx, scope)
	if err != nil {
		if errors.Is(err, collection.ErrForbidden) {
			return storecenter.ProductExecutionAuthorization{}, storecenter.ErrNotFound
		}
		return storecenter.ProductExecutionAuthorization{}, storecenter.ErrDependencyUnavailable
	}
	for _, permission := range permissions {
		allowed, err := authz.AuthorizeOrganization(ctx, a.Permissions, scope.ActorID, scope.OrganizationID, roles, permission)
		if err != nil {
			return storecenter.ProductExecutionAuthorization{}, storecenter.ErrDependencyUnavailable
		}
		if !allowed {
			return storecenter.ProductExecutionAuthorization{}, storecenter.ErrNotFound
		}
	}
	return storecenter.ProductExecutionAuthorization{Allowed: true, Access: storecenter.StoreMemberAccess{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, Administrator: a.Permissions.IsTenantAdmin("", roles), CanWrite: subject.Purpose != storecenter.ProductPurposeRules}}, nil
}

type OperationExecutionAuthorization struct {
	OrganizationExecutionAuthorizer
	Stores storecenter.ProductExecutionReader
}

func (a OperationExecutionAuthorization) AuthorizeOperationExecution(ctx context.Context, op preparation.Operation) error {
	roles, err := a.current(ctx, op.Owner)
	if err != nil {
		return err
	}
	purposes := []string{collection.PermissionRead, preparation.PermissionRead, preparation.PermissionManage}
	if op.Input.Action == preparation.OperationUpload {
		purposes = append(purposes, preparation.PermissionSubmit)
	}
	if op.Input.Action == preparation.OperationOptimize {
		purposes = append(purposes, authz.PermissionLocalAgentWrite, authz.PermissionWorkbenchAgentUse, authz.PermissionProductSourcingWrite)
	}
	for _, purpose := range purposes {
		allowed, err := authz.AuthorizeOrganization(ctx, a.Permissions, op.Owner.ActorID, op.Owner.OrganizationID, roles, purpose)
		if err != nil {
			return collection.ErrUnavailable
		}
		if !allowed {
			return collection.ErrForbidden
		}
	}
	if a.Stores == nil {
		return collection.ErrUnavailable
	}
	purpose := storecenter.ProductPurposeRules
	if op.Input.Action == preparation.OperationUpload {
		purpose = storecenter.ProductPurposePublish
	}
	_, err = a.Stores.ReadProductExecution(ctx, storecenter.ProductExecutionSubject{OrganizationID: op.Owner.OrganizationID, ActorID: op.Owner.ActorID, MemberID: op.Owner.MemberID, Purpose: purpose}, op.Input.StoreID, a.OrganizationExecutionAuthorizer, time.Now())
	if errors.Is(err, storecenter.ErrNotFound) {
		return collection.ErrForbidden
	}
	if err != nil {
		return collection.ErrUnavailable
	}
	return nil
}

var _ preparation.OperationExecutionAuthorizer = OperationExecutionAuthorization{}

var _ collection.ExecutionAuthorizer = OrganizationExecutionAuthorizer{}
var _ storecenter.ProductExecutionAuthorizer = OrganizationExecutionAuthorizer{}
