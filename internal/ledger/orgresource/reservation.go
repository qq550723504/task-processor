package orgresource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const OperationReserve = "reserve"

type ReservationState string

const (
	ReservationReserved               ReservationState = "reserved"
	ReservationCommitted              ReservationState = "committed"
	ReservationReleased               ReservationState = "released"
	ReservationReconciliationRequired ReservationState = "reconciliation_required"
)

type OwnerAttemptState string

const (
	OwnerAttemptNotStarted        OwnerAttemptState = "not_started"
	OwnerAttemptProcessing        OwnerAttemptState = "processing"
	OwnerAttemptOutcomeUnknown    OwnerAttemptState = "outcome_unknown"
	OwnerAttemptSucceededTerminal OwnerAttemptState = "succeeded_terminal"
	OwnerAttemptFailedTerminal    OwnerAttemptState = "failed_terminal"
	OwnerAttemptCancelledTerminal OwnerAttemptState = "cancelled_terminal"
)

var (
	ErrReservationOwnerNotRegistered = errors.New("organization resource reservation owner is not registered")
	ErrOwnerScopeMismatch            = errors.New("organization resource reservation owner scope mismatch")
	ErrOwnerNotReservable            = errors.New("organization resource reservation owner is not reservable")
	ErrInsufficientBalance           = errors.New("organization resource balance is insufficient")
)

type ReservationAuthorization struct {
	// MaxQuantity is supplied by the registered owner contract. A zero or
	// negative value closes admission instead of creating an unbounded generic
	// resource command.
	MaxQuantity int64
}

type ReservationAuthorizer interface {
	AuthorizeReservation(ctx context.Context, principal Principal, ownerType string, resourceType ResourceType) (ReservationAuthorization, error)
}

type ReservationExecutor interface {
	ReplayReservation(ctx context.Context, replay ReservationReplay) (ReservationResult, bool, error)
	ExecuteReservation(ctx context.Context, execution ReservationExecution) (ReservationResult, error)
}

type ReserveInput struct {
	OrganizationID     string
	OperationID        string
	OwnerType          string
	OwnerAttemptID     string
	BusinessScope      string
	ResourceType       ResourceType
	Quantity           int64
	ReservationPurpose string
	Principal          Principal
}

type ReservationReplay struct {
	OrganizationID     string
	OperationID        string
	OwnerType          string
	OwnerAttemptID     string
	BusinessScope      string
	ResourceType       ResourceType
	Quantity           int64
	ReservationPurpose string
	RequestFingerprint string
}

type ReservationExecution struct {
	OrganizationID     string
	OperationID        string
	OperationType      string
	OwnerType          string
	OwnerAttemptID     string
	BusinessScope      string
	ResourceType       ResourceType
	Quantity           int64
	ReservationPurpose string
	ActorID            string
	RequestFingerprint string
}

type ReservationSnapshot struct {
	OperationID        string           `json:"operation_id"`
	ReservationID      string           `json:"reservation_id"`
	OrganizationID     string           `json:"organization_id"`
	OwnerType          string           `json:"owner_type"`
	OwnerAttemptID     string           `json:"owner_attempt_id"`
	BusinessScope      string           `json:"business_scope"`
	ResourceType       ResourceType     `json:"resource_type"`
	Quantity           string           `json:"quantity"`
	ReservationPurpose string           `json:"reservation_purpose"`
	State              ReservationState `json:"state"`
	AvailableAfter     string           `json:"available_after"`
	ReservedAfter      string           `json:"reserved_after"`
	ConsumedAfter      string           `json:"consumed_after"`
	EventID            string           `json:"event_id"`
}

type ReservationResult struct {
	Snapshot ReservationSnapshot
	Replayed bool
}

type ReservationService struct {
	executor   ReservationExecutor
	authorizer ReservationAuthorizer
}

func NewReservationService(executor ReservationExecutor, authorizer ReservationAuthorizer) (*ReservationService, error) {
	if executor == nil {
		return nil, errors.New("reservation executor is required")
	}
	if authorizer == nil {
		return nil, errors.New("reservation authorizer is required")
	}
	return &ReservationService{executor: executor, authorizer: authorizer}, nil
}

// Reserve admits only a registered durable owner contract. Authorization is
// checked before durable replay; owner-policy validation happens after replay
// so a later rollout/configuration change cannot hide an already committed
// result.
func (service *ReservationService) Reserve(ctx context.Context, input ReserveInput) (ReservationResult, error) {
	if ctx == nil {
		return ReservationResult{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	input = normalizeReserveInput(input)
	if err := validateReserveIdentity(input); err != nil {
		return ReservationResult{}, err
	}
	authorization, err := service.authorizer.AuthorizeReservation(ctx, input.Principal, input.OwnerType, input.ResourceType)
	if err != nil {
		return ReservationResult{}, fmt.Errorf("%w: reservation principal or owner contract rejected", ErrForbidden)
	}
	execution := ReservationExecution{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: OperationReserve,
		OwnerType: input.OwnerType, OwnerAttemptID: input.OwnerAttemptID, BusinessScope: input.BusinessScope,
		ResourceType: input.ResourceType, Quantity: input.Quantity, ReservationPurpose: input.ReservationPurpose,
		ActorID: input.Principal.ID,
	}
	execution.RequestFingerprint, err = fingerprintReservation(execution)
	if err != nil {
		return ReservationResult{}, err
	}
	replay := ReservationReplay{
		OrganizationID: execution.OrganizationID, OperationID: execution.OperationID,
		OwnerType: execution.OwnerType, OwnerAttemptID: execution.OwnerAttemptID, BusinessScope: execution.BusinessScope,
		ResourceType: execution.ResourceType, Quantity: execution.Quantity, ReservationPurpose: execution.ReservationPurpose,
		RequestFingerprint: execution.RequestFingerprint,
	}
	if result, found, replayErr := service.executor.ReplayReservation(ctx, replay); replayErr != nil {
		return ReservationResult{}, replayErr
	} else if found {
		result.Replayed = true
		return result, nil
	}
	if authorization.MaxQuantity <= 0 || input.Quantity > authorization.MaxQuantity {
		return ReservationResult{}, fmt.Errorf("%w: quantity exceeds registered owner limit", ErrInvalidInput)
	}
	return service.executor.ExecuteReservation(ctx, execution)
}

func normalizeReserveInput(input ReserveInput) ReserveInput {
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.OwnerType = strings.TrimSpace(input.OwnerType)
	input.OwnerAttemptID = strings.TrimSpace(input.OwnerAttemptID)
	input.BusinessScope = strings.TrimSpace(input.BusinessScope)
	input.ReservationPurpose = strings.TrimSpace(input.ReservationPurpose)
	input.Principal.ID = strings.TrimSpace(input.Principal.ID)
	return input
}

func validateReserveIdentity(input ReserveInput) error {
	if input.Principal.ID == "" {
		return ErrForbidden
	}
	values := []struct {
		value string
		max   int
	}{
		{input.OrganizationID, 128},
		{input.OperationID, 128},
		{input.OwnerType, 96},
		{input.OwnerAttemptID, 192},
		{input.BusinessScope, 256},
		{input.ReservationPurpose, 96},
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

func validResourceType(resourceType ResourceType) bool {
	switch resourceType {
	case ResourceStoreRenewalPeriod, ResourceAIPoint, ResourceDataRow:
		return true
	default:
		return false
	}
}

func fingerprintReservation(input ReservationExecution) (string, error) {
	payload := struct {
		OperationType      string       `json:"operation_type"`
		OrganizationID     string       `json:"organization_id"`
		OwnerType          string       `json:"owner_type"`
		OwnerAttemptID     string       `json:"owner_attempt_id"`
		BusinessScope      string       `json:"business_scope"`
		ResourceType       ResourceType `json:"resource_type"`
		Quantity           int64        `json:"quantity"`
		ReservationPurpose string       `json:"reservation_purpose"`
	}{
		input.OperationType, input.OrganizationID, input.OwnerType, input.OwnerAttemptID,
		input.BusinessScope, input.ResourceType, input.Quantity, input.ReservationPurpose,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("fingerprint reservation: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
