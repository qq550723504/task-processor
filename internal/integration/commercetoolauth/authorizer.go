package commercetoolauth

import (
	"context"
	"errors"

	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
)

var errPermissionDenied = errors.New("permission denied")

// CasbinAuthorizer adapts the existing ListingKit policy owner to the
// provider-neutral Commerce Tool contract. It does not duplicate policy.
type CasbinAuthorizer struct {
	delegate *authz.ListingKitAuthorizer
}

func NewCasbinAuthorizer(delegate *authz.ListingKitAuthorizer) (*CasbinAuthorizer, error) {
	if delegate == nil {
		return nil, errors.New("listingkit authorizer is nil")
	}
	return &CasbinAuthorizer{delegate: delegate}, nil
}

func (a *CasbinAuthorizer) Authorize(ctx context.Context, principal commercetool.Principal, requirement commercetool.PermissionRequirement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a == nil || a.delegate == nil {
		return errPermissionDenied
	}
	if !a.delegate.Authorize(principal.UserID, append([]string(nil), principal.Roles...), requirement.Permission) {
		return errPermissionDenied
	}
	return nil
}
