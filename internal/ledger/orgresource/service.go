// Package orgresource owns organization-scoped platform resource balances.
// It is deliberately separate from subscription usage metering and money.
package orgresource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ResourceType string

const (
	ResourceStoreRenewalPeriod ResourceType = "store_renewal_period"
	ResourceAIPoint            ResourceType = "ai_point"
	ResourceDataRow            ResourceType = "data_row"
)

const (
	OperationGrantWelcomeStoreRenewalPeriod = "grant_welcome_store_renewal_period"
	SourceOnboardingWelcomeStorePeriod      = "onboarding_welcome_store_period"
)

type PrincipalKind string

const (
	PrincipalTenantHuman         PrincipalKind = "tenant_human"
	PrincipalTrustedProvisioning PrincipalKind = "trusted_provisioning"
)

var (
	ErrForbidden               = errors.New("organization resource command is forbidden")
	ErrInvalidInput            = errors.New("organization resource input is invalid")
	ErrWelcomeGrantNotApproved = errors.New("welcome resource grant is not approved")
	ErrIdempotencyKeyConflict  = errors.New("organization resource idempotency key conflict")
	ErrConcurrencyRetry        = errors.New("organization resource concurrency retry budget exhausted")
)

type Principal struct {
	ID   string
	Kind PrincipalKind
}

type WelcomeGrantApproval struct {
	OrganizationID string
	EvidenceID     string
	ApprovedAt     time.Time
}

type EligibilityVerifier interface {
	VerifyWelcomeGrantEligibility(ctx context.Context, organizationID string) (WelcomeGrantApproval, error)
}

// PrincipalAuthorizer is implemented by runtime assembly using a verified
// service identity. The domain never trusts a caller-provided role or kind as
// proof that the caller is the provisioning authority.
type PrincipalAuthorizer interface {
	AuthorizeWelcomeGrant(ctx context.Context, principal Principal) error
}

type WelcomeGrantExecutor interface {
	ReplayWelcomeGrant(ctx context.Context, input WelcomeGrantReplay) (WelcomeGrantResult, bool, error)
	ExecuteWelcomeGrant(ctx context.Context, input WelcomeGrantExecution) (WelcomeGrantResult, error)
}

type GrantWelcomeStoreRenewalPeriodInput struct {
	OrganizationID string
	OperationID    string
	Principal      Principal
}

type WelcomeGrantExecution struct {
	OrganizationID     string
	OperationID        string
	OperationType      string
	ResourceType       ResourceType
	Quantity           int64
	SourceType         string
	SourceIdentity     string
	ApprovalEvidenceID string
	ApprovedAt         time.Time
	ActorID            string
	RequestFingerprint string
}

type WelcomeGrantReplay struct {
	OrganizationID     string
	OperationID        string
	ResourceType       ResourceType
	SourceType         string
	SourceIdentity     string
	RequestFingerprint string
}

type WelcomeGrantSnapshot struct {
	OperationID    string       `json:"operation_id"`
	OrganizationID string       `json:"organization_id"`
	ResourceType   ResourceType `json:"resource_type"`
	Quantity       string       `json:"quantity"`
	BalanceAfter   string       `json:"balance_after"`
	SourceType     string       `json:"source_type"`
	SourceIdentity string       `json:"source_identity"`
	EventID        string       `json:"event_id"`
}

type WelcomeGrantResult struct {
	Snapshot WelcomeGrantSnapshot
	Replayed bool
}

type Service struct {
	executor   WelcomeGrantExecutor
	verifier   EligibilityVerifier
	authorizer PrincipalAuthorizer
}

func NewService(executor WelcomeGrantExecutor, verifier EligibilityVerifier, authorizer PrincipalAuthorizer) (*Service, error) {
	if executor == nil {
		return nil, errors.New("welcome grant executor is required")
	}
	if verifier == nil {
		return nil, errors.New("welcome grant eligibility verifier is required")
	}
	if authorizer == nil {
		return nil, errors.New("welcome grant principal authorizer is required")
	}
	return &Service{executor: executor, verifier: verifier, authorizer: authorizer}, nil
}

// GrantWelcomeStoreRenewalPeriod exposes the only Phase 1 positive-mint use
// case. Resource type, quantity, and source identity are fixed here so neither
// a browser nor a generic tenant caller can turn this into an arbitrary grant.
func (s *Service) GrantWelcomeStoreRenewalPeriod(ctx context.Context, input GrantWelcomeStoreRenewalPeriodInput) (WelcomeGrantResult, error) {
	if ctx == nil {
		return WelcomeGrantResult{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Principal.ID) == "" {
		return WelcomeGrantResult{}, ErrForbidden
	}
	if err := s.authorizer.AuthorizeWelcomeGrant(ctx, input.Principal); err != nil {
		return WelcomeGrantResult{}, fmt.Errorf("%w: principal rejected", ErrForbidden)
	}
	organizationID := strings.TrimSpace(input.OrganizationID)
	operationID := strings.TrimSpace(input.OperationID)
	if organizationID == "" || len(organizationID) > 128 || operationID == "" || len(operationID) > 128 {
		return WelcomeGrantResult{}, ErrInvalidInput
	}

	execution := WelcomeGrantExecution{
		OrganizationID: organizationID,
		OperationID:    operationID,
		OperationType:  OperationGrantWelcomeStoreRenewalPeriod,
		ResourceType:   ResourceStoreRenewalPeriod,
		Quantity:       1,
		SourceType:     SourceOnboardingWelcomeStorePeriod,
		SourceIdentity: organizationID,
		ActorID:        strings.TrimSpace(input.Principal.ID),
	}
	fingerprint, err := fingerprintWelcomeGrant(execution)
	if err != nil {
		return WelcomeGrantResult{}, err
	}
	execution.RequestFingerprint = fingerprint
	replayed, found, err := s.executor.ReplayWelcomeGrant(ctx, WelcomeGrantReplay{
		OrganizationID:     execution.OrganizationID,
		OperationID:        execution.OperationID,
		ResourceType:       execution.ResourceType,
		SourceType:         execution.SourceType,
		SourceIdentity:     execution.SourceIdentity,
		RequestFingerprint: execution.RequestFingerprint,
	})
	if err != nil {
		return WelcomeGrantResult{}, err
	}
	if found {
		replayed.Replayed = true
		return replayed, nil
	}

	approval, err := s.verifier.VerifyWelcomeGrantEligibility(ctx, organizationID)
	if err != nil {
		return WelcomeGrantResult{}, fmt.Errorf("verify welcome grant eligibility: %w", err)
	}
	approvalOrganizationID := strings.TrimSpace(approval.OrganizationID)
	evidenceID := strings.TrimSpace(approval.EvidenceID)
	if approvalOrganizationID != organizationID || evidenceID == "" || len(evidenceID) > 192 {
		return WelcomeGrantResult{}, ErrWelcomeGrantNotApproved
	}

	execution.ApprovalEvidenceID = evidenceID
	execution.ApprovedAt = approval.ApprovedAt.UTC()
	return s.executor.ExecuteWelcomeGrant(ctx, execution)
}

func fingerprintWelcomeGrant(input WelcomeGrantExecution) (string, error) {
	payload := struct {
		OperationType  string       `json:"operation_type"`
		OrganizationID string       `json:"organization_id"`
		ResourceType   ResourceType `json:"resource_type"`
		Quantity       string       `json:"quantity"`
		SourceType     string       `json:"source_type"`
		SourceIdentity string       `json:"source_identity"`
	}{
		OperationType:  input.OperationType,
		OrganizationID: input.OrganizationID,
		ResourceType:   input.ResourceType,
		Quantity:       "1",
		SourceType:     input.SourceType,
		SourceIdentity: input.SourceIdentity,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("fingerprint welcome grant: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
