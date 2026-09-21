package accountallocation

import (
	"context"
	"errors"
	"time"
)

const MetricToken = "token"

var (
	ErrInvalidRequest      = errors.New("account allocation invalid request")
	ErrNotFound            = errors.New("account allocation member not found")
	ErrConflict            = errors.New("account allocation version conflict")
	ErrIdempotencyConflict = errors.New("account allocation idempotency conflict")
	ErrQuotaExceeded       = errors.New("account allocation quota exceeded")
	ErrConsumedFloor       = errors.New("account allocation target below consumed")
	ErrAllocationRequired  = errors.New("account allocation required")
	ErrUnavailable         = errors.New("account allocation unavailable")
	ErrQuotaUnavailable    = errors.New("account allocation quota unavailable")
)

type Quota struct {
	OrganizationID string
	Metric         string
	Total          int64
	WindowStart    time.Time
	WindowEnd      time.Time
}

type Allocation struct {
	OrganizationID string    `json:"organizationId"`
	MemberID       string    `json:"memberId"`
	Metric         string    `json:"metric"`
	Allocated      int64     `json:"allocated"`
	Consumed       int64     `json:"consumed"`
	Remaining      int64     `json:"remaining"`
	Version        int64     `json:"version"`
	Active         bool      `json:"active"`
	WindowStart    time.Time `json:"windowStart"`
	WindowEnd      time.Time `json:"windowEnd"`
}

type EnterpriseView struct {
	Total       int64 `json:"total"`
	Allocated   int64 `json:"allocated"`
	Unallocated int64 `json:"unallocated"`
	Consumed    int64 `json:"consumed"`
}

type Snapshot struct {
	OrganizationID string         `json:"organizationId"`
	Metric         string         `json:"metric"`
	WindowStart    time.Time      `json:"windowStart"`
	WindowEnd      time.Time      `json:"windowEnd"`
	Enterprise     EnterpriseView `json:"enterprise"`
	Allocations    []Allocation   `json:"allocations"`
}

type AuditEvent struct {
	OrganizationID string
	ActorID        string
	MemberID       string
	Operation      string
	Target         int64
	Allocated      int64
	Consumed       int64
	Version        int64
	IdempotencyKey string
	CreatedAt      time.Time
}

// AuditPosition is the stable cursor for the commercial allocation audit
// stream. The idempotency key is unique and is used as the tie-breaker when
// two events share the same database timestamp.
type AuditPosition struct {
	CreatedAt      time.Time
	IdempotencyKey string
}

func (p AuditPosition) Valid() bool {
	return !p.CreatedAt.IsZero() && p.CreatedAt.Equal(p.CreatedAt.Truncate(time.Microsecond)) && p.IdempotencyKey != ""
}

func (e AuditEvent) Position() AuditPosition {
	return AuditPosition{CreatedAt: e.CreatedAt.UTC().Truncate(time.Microsecond), IdempotencyKey: e.IdempotencyKey}
}

type AuditPage struct {
	Items []AuditEvent
	Next  *AuditPosition
}

type SetTargetInput struct {
	OrganizationID  string
	MemberID        string
	Target          int64
	ExpectedVersion int64
	IdempotencyKey  string
	ActorID         string
}

type ConsumeInput struct {
	OrganizationID string
	MemberID       string
	Quantity       int64
	IdempotencyKey string
	SourceType     string
	SourceID       string
}

type Repository interface {
	Snapshot(context.Context, Quota) (Snapshot, error)
	SetTarget(context.Context, Quota, SetTargetInput) (Allocation, error)
	Consume(context.Context, Quota, ConsumeInput) error
}

// StaleAllocationReleaser is an optional commercial-owner operation used by
// the account boundary after it observes the live membership directory. It
// releases only unused reservations for members no longer present; membership
// remains the canonical owner of member state.
type StaleAllocationReleaser interface {
	RevokeMissingMembers(context.Context, Quota, []string, string) error
}

type QuotaReader interface {
	ReadTokenQuota(context.Context, string) (Quota, error)
}

type Service struct {
	quotas QuotaReader
	repo   Repository
}

func NewService(quotas QuotaReader, repo Repository) (*Service, error) {
	if quotas == nil || repo == nil {
		return nil, ErrUnavailable
	}
	return &Service{quotas: quotas, repo: repo}, nil
}

func (s *Service) Snapshot(ctx context.Context, organizationID string) (Snapshot, error) {
	if s == nil || s.quotas == nil || s.repo == nil || organizationID == "" {
		return Snapshot{}, ErrInvalidRequest
	}
	quota, err := s.quotas.ReadTokenQuota(ctx, organizationID)
	if err != nil {
		return Snapshot{}, err
	}
	return s.repo.Snapshot(ctx, quota)
}

func (s *Service) SetTarget(ctx context.Context, input SetTargetInput) (Allocation, error) {
	if s == nil || s.quotas == nil || s.repo == nil || input.OrganizationID == "" || input.MemberID == "" || input.IdempotencyKey == "" || input.ActorID == "" || input.ExpectedVersion < 0 || input.Target < 0 {
		return Allocation{}, ErrInvalidRequest
	}
	quota, err := s.quotas.ReadTokenQuota(ctx, input.OrganizationID)
	if err != nil {
		return Allocation{}, err
	}
	return s.repo.SetTarget(ctx, quota, input)
}

func (s *Service) Consume(ctx context.Context, input ConsumeInput) error {
	if s == nil || s.quotas == nil || s.repo == nil || input.OrganizationID == "" || input.MemberID == "" || input.IdempotencyKey == "" || input.SourceType == "" || input.SourceID == "" || input.Quantity <= 0 {
		return ErrInvalidRequest
	}
	quota, err := s.quotas.ReadTokenQuota(ctx, input.OrganizationID)
	if err != nil {
		return err
	}
	return s.repo.Consume(ctx, quota, input)
}

func (s *Service) RevokeMissingMembers(ctx context.Context, organizationID string, activeMemberIDs []string, actorID string) error {
	if s == nil || s.quotas == nil || s.repo == nil || organizationID == "" || actorID == "" {
		return ErrInvalidRequest
	}
	releaser, ok := s.repo.(StaleAllocationReleaser)
	if !ok {
		return ErrUnavailable
	}
	quota, err := s.quotas.ReadTokenQuota(ctx, organizationID)
	if err != nil {
		return err
	}
	return releaser.RevokeMissingMembers(ctx, quota, activeMemberIDs, actorID)
}
