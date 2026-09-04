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

const OperationSettleReservation = "settle_reservation"

type SettlementDecision string

const (
	SettlementCommit  SettlementDecision = "commit"
	SettlementRelease SettlementDecision = "release"
)

var (
	ErrOwnerNotTerminal    = errors.New("organization resource reservation owner is not terminal")
	ErrInvalidOwnerProof   = errors.New("organization resource reservation owner proof is invalid")
	ErrReservationNotFound = errors.New("organization resource reservation was not found")
)

type SettlementAuthorizer interface {
	AuthorizeSettlement(ctx context.Context, principal Principal) error
}

type SettlementExecutor interface {
	ReplaySettlement(ctx context.Context, replay SettlementReplay) (SettlementResult, bool, error)
	ExecuteSettlement(ctx context.Context, execution SettlementExecution) (SettlementResult, error)
}

type SettlementInput struct {
	OrganizationID string
	OperationID    string
	ReservationID  string
	Principal      Principal
}

type SettlementReplay struct {
	OrganizationID     string
	OperationID        string
	ReservationID      string
	RequestFingerprint string
}

type SettlementExecution struct {
	OrganizationID     string
	OperationID        string
	OperationType      string
	ReservationID      string
	ActorID            string
	RequestFingerprint string
}

type SettlementSnapshot struct {
	OperationID        string             `json:"operation_id"`
	ReservationID      string             `json:"reservation_id"`
	OrganizationID     string             `json:"organization_id"`
	OwnerType          string             `json:"owner_type"`
	OwnerAttemptID     string             `json:"owner_attempt_id"`
	BusinessScope      string             `json:"business_scope"`
	ResourceType       ResourceType       `json:"resource_type"`
	Quantity           string             `json:"quantity"`
	ReservationPurpose string             `json:"reservation_purpose"`
	Decision           SettlementDecision `json:"decision"`
	OwnerTerminalState OwnerAttemptState  `json:"owner_terminal_state"`
	OwnerEvidenceID    string             `json:"owner_evidence_id"`
	GrossCredit        string             `json:"gross_credit"`
	DebtRepaid         string             `json:"debt_repaid"`
	NetCredit          string             `json:"net_credit"`
	AvailableAfter     string             `json:"available_after"`
	ReservedAfter      string             `json:"reserved_after"`
	ConsumedAfter      string             `json:"consumed_after"`
	EventID            string             `json:"event_id"`
}

type SettlementResult struct {
	Snapshot SettlementSnapshot
	Replayed bool
}

type SettlementService struct {
	executor   SettlementExecutor
	authorizer SettlementAuthorizer
}

func NewSettlementService(executor SettlementExecutor, authorizer SettlementAuthorizer) (*SettlementService, error) {
	if executor == nil {
		return nil, errors.New("settlement executor is required")
	}
	if authorizer == nil {
		return nil, errors.New("settlement authorizer is required")
	}
	return &SettlementService{executor: executor, authorizer: authorizer}, nil
}

// Settle accepts no caller-controlled outcome. The persistence adapter derives
// commit versus release from a transactionally locked terminal owner proof.
func (service *SettlementService) Settle(ctx context.Context, input SettlementInput) (SettlementResult, error) {
	if ctx == nil {
		return SettlementResult{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.ReservationID = strings.TrimSpace(input.ReservationID)
	input.Principal.ID = strings.TrimSpace(input.Principal.ID)
	if input.Principal.ID == "" {
		return SettlementResult{}, ErrForbidden
	}
	if input.OrganizationID == "" || len(input.OrganizationID) > 128 || input.OperationID == "" || len(input.OperationID) > 128 || input.ReservationID == "" || len(input.ReservationID) > 128 {
		return SettlementResult{}, ErrInvalidInput
	}
	if err := service.authorizer.AuthorizeSettlement(ctx, input.Principal); err != nil {
		return SettlementResult{}, fmt.Errorf("%w: settlement principal rejected", ErrForbidden)
	}
	execution := SettlementExecution{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: OperationSettleReservation,
		ReservationID: input.ReservationID, ActorID: input.Principal.ID,
	}
	fingerprint, err := fingerprintSettlement(execution)
	if err != nil {
		return SettlementResult{}, err
	}
	execution.RequestFingerprint = fingerprint
	replay := SettlementReplay{
		OrganizationID: execution.OrganizationID, OperationID: execution.OperationID,
		ReservationID: execution.ReservationID, RequestFingerprint: execution.RequestFingerprint,
	}
	if result, found, replayErr := service.executor.ReplaySettlement(ctx, replay); replayErr != nil {
		return SettlementResult{}, replayErr
	} else if found {
		result.Replayed = true
		return result, nil
	}
	return service.executor.ExecuteSettlement(ctx, execution)
}

func fingerprintSettlement(input SettlementExecution) (string, error) {
	payload := struct {
		OperationType  string `json:"operation_type"`
		OrganizationID string `json:"organization_id"`
		ReservationID  string `json:"reservation_id"`
	}{input.OperationType, input.OrganizationID, input.ReservationID}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("fingerprint settlement: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
