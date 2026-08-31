package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const storeQuotaMetric = "store_count"

var (
	ErrStoreQuotaInvalidInput      = errors.New("store quota invalid input")
	ErrStoreQuotaNotFound          = errors.New("store quota allocation not found")
	ErrStoreQuotaIdentityMismatch  = errors.New("store quota allocation identity mismatch")
	ErrStoreQuotaInvalidTransition = errors.New("store quota invalid transition")
	ErrStoreQuotaExceeded          = errors.New("store quota exceeded")
	ErrStoreQuotaNotConfigured     = errors.New("store quota ledger is not configured")
)

type StoreQuotaAllocationStatus string

const (
	StoreQuotaReserved  StoreQuotaAllocationStatus = "reserved"
	StoreQuotaAllocated StoreQuotaAllocationStatus = "allocated"
	StoreQuotaReleased  StoreQuotaAllocationStatus = "released"
)

type StoreQuotaReserveInput struct {
	OrganizationID string
	RequestKey     string
	ActorSubject   string
}

type StoreQuotaTransitionInput struct {
	OrganizationID string
	AllocationID   string
	StoreID        string
	RequestKey     string
	ActorSubject   string
}

type StoreQuotaAllocation struct {
	OrganizationID string
	AllocationID   string
	StoreID        string
	RequestKey     string
	Status         StoreQuotaAllocationStatus
	CreatedBy      string
	UpdatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	AllocatedAt    *time.Time
	ReleasedAt     *time.Time
}

type StoreQuotaReserveResult struct {
	Allocation   StoreQuotaAllocation
	AllocationID string
	StoreID      string
	Existing     bool
}

type StoreQuotaTransitionResult struct {
	Allocation StoreQuotaAllocation
	Existing   bool
}

type StoreQuotaSummary struct {
	OrganizationID string
	Committed      int64
	Reserved       int64
	Limit          *int64
	Allowed        bool
	Reason         string
}

// StoreQuotaLedger owns durable Store allocation admission independently from
// the generic usage ledger. Store Center receives only this domain contract.
type StoreQuotaLedger interface {
	Reserve(context.Context, StoreQuotaReserveInput) (StoreQuotaReserveResult, error)
	Commit(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error)
	ReleaseReservation(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error)
	Deallocate(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error)
	GetByRequestKey(context.Context, string, string) (*StoreQuotaAllocation, error)
	Summary(context.Context, string) (StoreQuotaSummary, error)
}

type StoreQuotaValidationError struct{ Field string }

func (e *StoreQuotaValidationError) Error() string {
	return fmt.Sprintf("store quota invalid %s", e.Field)
}
func (e *StoreQuotaValidationError) Is(target error) bool { return target == ErrStoreQuotaInvalidInput }

type StoreQuotaExceededError struct {
	OrganizationID string
	Committed      int64
	Reserved       int64
	Limit          int64
}

func (e *StoreQuotaExceededError) Error() string {
	return fmt.Sprintf("store quota exceeded for organization %q: committed=%d reserved=%d limit=%d", e.OrganizationID, e.Committed, e.Reserved, e.Limit)
}
func (e *StoreQuotaExceededError) Is(target error) bool { return target == ErrStoreQuotaExceeded }

func NormalizeAndValidateStoreQuotaReserveInput(input StoreQuotaReserveInput) (StoreQuotaReserveInput, error) {
	if err := validateStoreQuotaText(input.OrganizationID, "organization_id"); err != nil {
		return StoreQuotaReserveInput{}, err
	}
	if err := validateStoreQuotaUUID(input.RequestKey, "request_key"); err != nil {
		return StoreQuotaReserveInput{}, err
	}
	if err := validateStoreQuotaText(input.ActorSubject, "actor_subject"); err != nil {
		return StoreQuotaReserveInput{}, err
	}
	return input, nil
}

func normalizeAndValidateStoreQuotaTransitionInput(input StoreQuotaTransitionInput) (StoreQuotaTransitionInput, error) {
	if err := validateStoreQuotaText(input.OrganizationID, "organization_id"); err != nil {
		return StoreQuotaTransitionInput{}, err
	}
	for field, value := range map[string]string{"allocation_id": input.AllocationID, "store_id": input.StoreID, "request_key": input.RequestKey} {
		if err := validateStoreQuotaUUID(value, field); err != nil {
			return StoreQuotaTransitionInput{}, err
		}
	}
	if err := validateStoreQuotaText(input.ActorSubject, "actor_subject"); err != nil {
		return StoreQuotaTransitionInput{}, err
	}
	return input, nil
}

func validateStoreQuotaText(value, field string) error {
	if value == "" || len(value) > 200 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return &StoreQuotaValidationError{Field: field}
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return &StoreQuotaValidationError{Field: field}
		}
	}
	return nil
}

func validateStoreQuotaUUID(value, field string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return &StoreQuotaValidationError{Field: field}
	}
	return nil
}
