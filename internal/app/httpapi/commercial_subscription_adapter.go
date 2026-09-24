package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	"task-processor/internal/listingsubscription"
)

type purchasedSubscriptionOwnerAdapter struct {
	owner *listingsubscription.Service
}

func (adapter purchasedSubscriptionOwnerAdapter) ResolvePurchasablePlan(ctx context.Context, planCode string) (billing.SubscriptionPlanSnapshot, error) {
	plan, err := adapter.owner.ResolvePurchasablePlan(ctx, planCode)
	if err != nil {
		if errors.Is(err, listingsubscription.ErrPurchasablePlanUnavailable) {
			return billing.SubscriptionPlanSnapshot{}, billing.ErrOfferUnavailable
		}
		return billing.SubscriptionPlanSnapshot{}, err
	}
	return billing.SubscriptionPlanSnapshot{PlanCode: plan.PlanCode, DisplayName: plan.DisplayName, Fingerprint: plan.Fingerprint}, nil
}

func (adapter purchasedSubscriptionOwnerAdapter) ReadPurchasedSubscriptionState(ctx context.Context, organizationID string) (billing.SubscriptionPurchaseState, error) {
	state, err := adapter.owner.ReadPurchasedSubscriptionState(ctx, organizationID)
	if err != nil {
		return billing.SubscriptionPurchaseState{}, err
	}
	return billing.SubscriptionPurchaseState{PlanCode: state.PlanCode, BlocksPurchase: state.BlocksPurchase}, nil
}

func (adapter purchasedSubscriptionOwnerAdapter) ActivatePurchasedSubscription(ctx context.Context, request billing.SubscriptionActivationRequest) (billing.SubscriptionActivationResult, error) {
	result, err := adapter.owner.ActivatePurchasedPlan(ctx, listingsubscription.PurchasedPlanActivationInput{OperationID: request.OperationID, OrganizationID: request.OrganizationID, ActorID: request.ActorID, SourceType: listingsubscription.PurchasedPlanSourceCommercialOrder, SourceID: request.CommercialOrderID, PlanCode: request.PlanCode, PlanFingerprint: request.PlanFingerprint, TermMonths: request.TermMonths})
	if err != nil {
		return billing.SubscriptionActivationResult{}, err
	}
	return mapPurchasedActivation(result), nil
}

func (adapter purchasedSubscriptionOwnerAdapter) ReadPurchasedSubscriptionActivation(ctx context.Context, organizationID, orderID string) (billing.SubscriptionActivationResult, error) {
	result, err := adapter.owner.ReadPurchasedPlanActivation(ctx, organizationID, orderID)
	if errors.Is(err, listingsubscription.ErrPurchasedPlanActivationNotFound) {
		return billing.SubscriptionActivationResult{}, billing.ErrSubscriptionActivationNotFound
	}
	if err != nil {
		return billing.SubscriptionActivationResult{}, err
	}
	return mapPurchasedActivation(result), nil
}

func mapPurchasedActivation(result listingsubscription.PurchasedPlanActivationResult) billing.SubscriptionActivationResult {
	subscriptionID := ""
	if result.SubscriptionID > 0 {
		subscriptionID = strconv.FormatInt(result.SubscriptionID, 10)
	}
	return billing.SubscriptionActivationResult{OperationID: result.OperationID, OrganizationID: result.OrganizationID, CommercialOrderID: result.SourceID, PlanCode: result.PlanCode, PlanFingerprint: result.PlanFingerprint, ActivationRequestFingerprint: result.ActivationRequestFingerprint, Outcome: billing.SubscriptionActivationOutcome(result.Outcome), FailureCode: billing.SubscriptionActivationFailureCode(result.FailureCode), SubscriptionID: subscriptionID, StartsAt: result.StartsAt, ExpiresAt: result.ExpiresAt, EntitlementSetFingerprint: result.EntitlementSetFingerprint, DecidedAt: result.DecidedAt, Existing: result.Existing}
}

type exactServiceAuthorizationReader interface {
	ReadExactServiceProjectAuthorization(context.Context, string, string, string, string) (zitadelruntime.ExactServiceProjectAuthorization, error)
}

type subscriptionPurchaseRecoveryAuthorizer struct {
	reader       exactServiceAuthorizationReader
	serviceToken string
	projectID    string
	authorizer   *authz.ListingKitAuthorizer
}

func (adapter subscriptionPurchaseRecoveryAuthorizer) ReauthorizeCommercialPurchase(ctx context.Context, organizationID, actorID string) (billing.CommercialPurchaseAuthorization, error) {
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	if adapter.reader == nil || adapter.authorizer == nil || strings.TrimSpace(adapter.serviceToken) == "" || strings.TrimSpace(adapter.projectID) == "" || organizationID == "" || actorID == "" {
		return billing.CommercialPurchaseAuthorization{}, billing.ErrFeatureUnavailable
	}
	result, err := adapter.reader.ReadExactServiceProjectAuthorization(ctx, adapter.serviceToken, actorID, adapter.projectID, organizationID)
	if err != nil {
		return billing.CommercialPurchaseAuthorization{}, err
	}
	allowed := result.Found && result.State == "STATE_ACTIVE" && adapter.authorizer.Authorize(actorID, result.Roles, authz.PermissionWorkbenchCommercialPurchase)
	return billing.CommercialPurchaseAuthorization{OrganizationID: organizationID, ActorID: actorID, Roles: append([]string(nil), result.Roles...), Allowed: allowed, ObservedAt: time.Now().UTC()}, nil
}
