package orgresource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	OperationGrantCommercialPurchase = "grant_commercial_purchase"
	SourceCommercialOrderItem        = "commercial_order_item"
	PrincipalTrustedCommercial       PrincipalKind = "trusted_commercial"
)

type PurchasedGrantAuthorization struct {
	MaxQuantity int64
}

// PurchasedGrantAuthorizer is supplied by runtime assembly. The resource
// domain never accepts a browser role or caller-provided PrincipalKind as proof
// that the caller is the canonical commercial owner.
type PurchasedGrantAuthorizer interface {
	AuthorizePurchasedGrant(context.Context, Principal, ResourceType) (PurchasedGrantAuthorization, error)
}

type PurchasedResourceGrantInput struct {
	OrganizationID       string
	OperationID          string
	CommercialOrderID    string
	CommercialOrderItemID string
	ResourceType         ResourceType
	Quantity             int64
	Principal            Principal
}

type PurchasedResourceGrantReplay struct {
	OrganizationID       string
	OperationID          string
	CommercialOrderID    string
	CommercialOrderItemID string
	ResourceType         ResourceType
	Quantity             int64
	SourceType           string
	SourceIdentity       string
	RequestFingerprint  string
}

type PurchasedResourceGrantExecution struct {
	OrganizationID       string
	OperationID          string
	OperationType        string
	CommercialOrderID    string
	CommercialOrderItemID string
	ResourceType         ResourceType
	Quantity             int64
	SourceType           string
	SourceIdentity       string
	ActorID              string
	RequestFingerprint  string
}

type PurchasedResourceGrantSnapshot struct {
	OperationID          string       `json:"operation_id"`
	OrganizationID       string       `json:"organization_id"`
	CommercialOrderID    string       `json:"commercial_order_id"`
	CommercialOrderItemID string       `json:"commercial_order_item_id"`
	ResourceType         ResourceType `json:"resource_type"`
	Quantity             string       `json:"quantity"`
	BalanceAfter         string       `json:"balance_after"`
	SourceType           string       `json:"source_type"`
	SourceIdentity       string       `json:"source_identity"`
	EventID              string       `json:"event_id"`
}

type PurchasedResourceGrantResult struct {
	Snapshot PurchasedResourceGrantSnapshot
	Replayed bool
}

type PurchasedResourceGrantExecutor interface {
	ReplayPurchasedResourceGrant(context.Context, PurchasedResourceGrantReplay) (PurchasedResourceGrantResult, bool, error)
	ExecutePurchasedResourceGrant(context.Context, PurchasedResourceGrantExecution) (PurchasedResourceGrantResult, error)
}

type PurchasedResourceGrantService struct {
	executor   PurchasedResourceGrantExecutor
	authorizer PurchasedGrantAuthorizer
}

func NewPurchasedResourceGrantService(executor PurchasedResourceGrantExecutor, authorizer PurchasedGrantAuthorizer) (*PurchasedResourceGrantService, error) {
	if executor == nil {
		return nil, fmt.Errorf("%w: purchased grant executor is required", ErrInvalidInput)
	}
	if authorizer == nil {
		return nil, fmt.Errorf("%w: purchased grant authorizer is required", ErrInvalidInput)
	}
	return &PurchasedResourceGrantService{executor: executor, authorizer: authorizer}, nil
}

// GrantPurchasedResource is the only commercial positive-mint contract. Source
// type and identity are derived from the canonical order/item and cannot be
// replaced with a caller-selected generic grant source.
func (service *PurchasedResourceGrantService) GrantPurchasedResource(ctx context.Context, input PurchasedResourceGrantInput) (PurchasedResourceGrantResult, error) {
	if ctx == nil {
		return PurchasedResourceGrantResult{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	input = normalizePurchasedGrantInput(input)
	if err := validatePurchasedGrantIdentity(input); err != nil {
		return PurchasedResourceGrantResult{}, err
	}
	authorization, err := service.authorizer.AuthorizePurchasedGrant(ctx, input.Principal, input.ResourceType)
	if err != nil {
		return PurchasedResourceGrantResult{}, fmt.Errorf("%w: commercial grant authority rejected", ErrForbidden)
	}
	if authorization.MaxQuantity <= 0 || input.Quantity > authorization.MaxQuantity {
		return PurchasedResourceGrantResult{}, fmt.Errorf("%w: quantity exceeds commercial grant limit", ErrInvalidInput)
	}

	execution := PurchasedResourceGrantExecution{
		OrganizationID:        input.OrganizationID,
		OperationID:           input.OperationID,
		OperationType:         OperationGrantCommercialPurchase,
		CommercialOrderID:     input.CommercialOrderID,
		CommercialOrderItemID: input.CommercialOrderItemID,
		ResourceType:          input.ResourceType,
		Quantity:              input.Quantity,
		SourceType:            SourceCommercialOrderItem,
		SourceIdentity:        purchasedGrantSourceIdentity(input.CommercialOrderID, input.CommercialOrderItemID),
		ActorID:               input.Principal.ID,
	}
	execution.RequestFingerprint, err = fingerprintPurchasedResourceGrant(execution)
	if err != nil {
		return PurchasedResourceGrantResult{}, err
	}

	replay := PurchasedResourceGrantReplay{
		OrganizationID:        execution.OrganizationID,
		OperationID:           execution.OperationID,
		CommercialOrderID:     execution.CommercialOrderID,
		CommercialOrderItemID: execution.CommercialOrderItemID,
		ResourceType:          execution.ResourceType,
		Quantity:              execution.Quantity,
		SourceType:            execution.SourceType,
		SourceIdentity:        execution.SourceIdentity,
		RequestFingerprint:   execution.RequestFingerprint,
	}
	if result, found, replayErr := service.executor.ReplayPurchasedResourceGrant(ctx, replay); replayErr != nil {
		return PurchasedResourceGrantResult{}, replayErr
	} else if found {
		result.Replayed = true
		return result, nil
	}
	return service.executor.ExecutePurchasedResourceGrant(ctx, execution)
}

func normalizePurchasedGrantInput(input PurchasedResourceGrantInput) PurchasedResourceGrantInput {
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.CommercialOrderID = strings.TrimSpace(input.CommercialOrderID)
	input.CommercialOrderItemID = strings.TrimSpace(input.CommercialOrderItemID)
	input.Principal.ID = strings.TrimSpace(input.Principal.ID)
	return input
}

func validatePurchasedGrantIdentity(input PurchasedResourceGrantInput) error {
	if input.Principal.ID == "" {
		return ErrForbidden
	}
	values := []struct {
		value string
		max   int
	}{
		{input.OrganizationID, 128},
		{input.OperationID, 128},
		{input.CommercialOrderID, 128},
		{input.CommercialOrderItemID, 128},
	}
	for _, candidate := range values {
		if candidate.value == "" || len(candidate.value) > candidate.max {
			return ErrInvalidInput
		}
	}
	if input.Quantity <= 0 || !validResourceType(input.ResourceType) {
		return ErrInvalidInput
	}
	return nil
}

func purchasedGrantSourceIdentity(orderID, orderItemID string) string {
	return orderID + ":" + orderItemID
}

func fingerprintPurchasedResourceGrant(input PurchasedResourceGrantExecution) (string, error) {
	payload := struct {
		OperationType         string       `json:"operation_type"`
		OrganizationID        string       `json:"organization_id"`
		CommercialOrderID     string       `json:"commercial_order_id"`
		CommercialOrderItemID string       `json:"commercial_order_item_id"`
		ResourceType          ResourceType `json:"resource_type"`
		Quantity              int64        `json:"quantity"`
		SourceType            string       `json:"source_type"`
		SourceIdentity        string       `json:"source_identity"`
	}{
		input.OperationType,
		input.OrganizationID,
		input.CommercialOrderID,
		input.CommercialOrderItemID,
		input.ResourceType,
		input.Quantity,
		input.SourceType,
		input.SourceIdentity,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("fingerprint purchased resource grant: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
