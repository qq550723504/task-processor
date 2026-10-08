package httpapi

import (
	"context"
	"strings"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	"time"
)

type exactServiceAuthorizationReader interface {
	ReadExactServiceProjectAuthorization(context.Context, string, string, string, string) (zitadelruntime.ExactServiceProjectAuthorization, error)
}

type financialRecoveryAuthorizer struct {
	reader       exactServiceAuthorizationReader
	serviceToken string
	projectID    string
	authorizer   *authz.ListingKitAuthorizer
}

func (adapter financialRecoveryAuthorizer) ReauthorizeCommercialPurchase(ctx context.Context, organizationID, actorID string) (billing.CommercialPurchaseAuthorization, error) {
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if adapter.reader == nil || adapter.authorizer == nil || strings.TrimSpace(adapter.serviceToken) == "" || strings.TrimSpace(adapter.projectID) == "" || organizationID == "" || actorID == "" {
		return billing.CommercialPurchaseAuthorization{}, billing.ErrFeatureUnavailable
	}
	result, err := adapter.reader.ReadExactServiceProjectAuthorization(ctx, adapter.serviceToken, actorID, adapter.projectID, organizationID)
	if err != nil {
		return billing.CommercialPurchaseAuthorization{}, err
	}
	allowed := result.Found && result.State == "STATE_ACTIVE" && authz.AllowedOrganization(ctx, adapter.authorizer, actorID, organizationID, result.Roles, authz.PermissionWorkbenchCommercialPurchase)
	return billing.CommercialPurchaseAuthorization{OrganizationID: organizationID, ActorID: actorID, Roles: append([]string(nil), result.Roles...), Allowed: allowed, ObservedAt: time.Now().UTC()}, nil
}
