package sourceaccountregistry

import (
	"context"
	"time"
	"unicode"

	"task-processor/internal/authz"
)

// CommittedOperation is the immutable receipt written by the current registry
// transaction. It is not an attempt/security log. OccurredAt is the owner's
// operation time, not a database commit timestamp or a global causal sequence.
type CommittedOperation struct {
	OrganizationID string
	AccountID      string
	ActorSubject   string
	Kind           OperationKind
	Version        int64
	OccurredAt     time.Time
}
type HistoryPosition struct {
	OccurredAt time.Time
	AccountID  string
	Version    int64
}
type HistoryRequest struct {
	Limit int
	After *HistoryPosition
}
type HistoryPage struct {
	Items []CommittedOperation
	Next  *HistoryPosition
}
type HistoryReader interface {
	ListCommittedOperations(context.Context, string, HistoryRequest) (HistoryPage, error)
}
type HistoryService struct {
	source *Service
	reader HistoryReader
}

// NewHistoryService reuses the registry's existing identity/permission boundary.
// It grants no writes and does not add history methods to the mutation store.
func NewHistoryService(source *Service, reader HistoryReader) (*HistoryService, error) {
	if source == nil || source.authorizer == nil || source.now == nil || isNilInterface(reader) {
		return nil, ErrUnavailable
	}
	return &HistoryService{source: source, reader: reader}, nil
}
func (s *HistoryService) List(ctx context.Context, request HistoryRequest) (HistoryPage, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.source.authorize(ctx, authz.PermissionWorkbenchSourceAccountRead)
	if err != nil {
		return HistoryPage{}, err
	}
	if request.Validate() != nil {
		return HistoryPage{}, ErrInvalid
	}
	page, err := s.reader.ListCommittedOperations(ctx, scope.OrganizationID, request)
	if err != nil {
		return HistoryPage{}, err
	}
	if err = ctx.Err(); err != nil {
		return HistoryPage{}, err
	}
	if len(page.Items) > request.Limit {
		return HistoryPage{}, ErrUnavailable
	}
	previous := request.After
	for _, item := range page.Items {
		if item.Validate() != nil || item.OrganizationID != scope.OrganizationID {
			return HistoryPage{}, ErrUnavailable
		}
		position := item.Position()
		if previous != nil && !position.Before(*previous) {
			return HistoryPage{}, ErrUnavailable
		}
		previous = &position
	}
	if page.Next != nil && (len(page.Items) != request.Limit || page.Next.Validate() != nil || previous == nil || !page.Next.Equal(*previous)) {
		return HistoryPage{}, ErrUnavailable
	}
	if page.Items == nil {
		page.Items = []CommittedOperation{}
	}
	return page, nil
}
func (r HistoryRequest) Validate() error {
	if r.Limit < 1 || r.Limit > MaxPageLimit || r.After != nil && r.After.Validate() != nil {
		return ErrInvalid
	}
	return nil
}
func (p HistoryPosition) Validate() error {
	if p.OccurredAt.IsZero() || !p.OccurredAt.Equal(p.OccurredAt.Truncate(time.Microsecond)) || !validUUIDv7(p.AccountID) || p.Version < 1 {
		return ErrInvalid
	}
	return nil
}
func (p HistoryPosition) Equal(other HistoryPosition) bool {
	return p.OccurredAt.Equal(other.OccurredAt) && p.AccountID == other.AccountID && p.Version == other.Version
}
func (p HistoryPosition) Before(other HistoryPosition) bool {
	if !p.OccurredAt.Equal(other.OccurredAt) {
		return p.OccurredAt.Before(other.OccurredAt)
	}
	if p.AccountID != other.AccountID {
		return p.AccountID < other.AccountID
	}
	return p.Version < other.Version
}
func (r CommittedOperation) Position() HistoryPosition {
	return HistoryPosition{OccurredAt: r.OccurredAt, AccountID: r.AccountID, Version: r.Version}
}
func (r CommittedOperation) Validate() error {
	if !validScopeValue(r.OrganizationID, MaxOrganizationIDBytes) || !validScopeValue(r.ActorSubject, MaxActorSubjectBytes) || r.Position().Validate() != nil {
		return ErrUnavailable
	}
	for _, ch := range r.ActorSubject {
		if unicode.IsControl(ch) {
			return ErrUnavailable
		}
	}
	switch r.Kind {
	case OperationRegister:
		if r.Version != 1 {
			return ErrUnavailable
		}
	case OperationEnable, OperationDisable:
		if r.Version < 2 {
			return ErrUnavailable
		}
	default:
		return ErrUnavailable
	}
	return nil
}
