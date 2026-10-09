package review

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"time"
)

// NotificationRecordReader is a pure keyset port; it includes terminal rows
// without changing the existing actionable review collection.
type NotificationRecordReader interface {
	ListNotificationRecords(context.Context, Scope, string, int) ([]Record, string, error)
}

func (s *Service) NotificationFacts(ctx context.Context, after string, limit int) ([]Record, string, error) {
	if s == nil {
		return nil, "", ErrUnavailable
	}
	scope, e := s.authorize(ctx, false, false)
	if e != nil {
		return nil, "", e
	}
	if limit < 1 || limit > 100 || (after != "" && !validID(after)) {
		return nil, "", ErrInvalid
	}
	reader, ok := s.store.(NotificationRecordReader)
	if !ok {
		return nil, "", ErrUnavailable
	}
	return reader.ListNotificationRecords(ctx, scope, after, limit)
}
func (s *Service) NotificationFact(ctx context.Context, id string) (Record, error) {
	scope, e := s.authorize(ctx, false, false)
	if e != nil {
		return Record{}, e
	}
	if !validID(id) {
		return Record{}, ErrInvalid
	}
	return s.store.Read(ctx, scope, id)
}

type TaskStateMetadata struct {
	State      string
	Revision   uint64
	OccurredAt *time.Time
}

func (r Record) TaskStateMetadata() TaskStateMetadata {
	out := TaskStateMetadata{State: r.State, Revision: r.Revision}
	if r.Receipt != nil {
		at := r.Receipt.At
		out.OccurredAt = &at
	} else if len(r.History) > 0 {
		at := r.History[len(r.History)-1].At
		out.OccurredAt = &at
	}
	return out
}

// This port returns no Review ID, title, evidence or product identity.
func (s *Service) FindAgentTaskReviewMetadata(ctx context.Context, runID string) (TaskStateMetadata, bool, error) {
	if s == nil || ctx == nil || !ValidKey(runID) {
		return TaskStateMetadata{}, false, ErrInvalid
	}
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !ValidKey(i.UserID) || i.TenantID != i.EffectiveOrganizationID || !time.Now().Before(i.TokenExpiresAt) {
		return TaskStateMetadata{}, false, ErrForbidden
	}
	allowed, err := authz.AuthorizeOrganization(ctx, s.auth, i.UserID, i.TenantID, i.Roles, authz.PermissionWorkbenchTaskRead)
	if err != nil {
		return TaskStateMetadata{}, false, ErrUnavailable
	}
	if !allowed {
		return TaskStateMetadata{}, false, ErrForbidden
	}
	r, found, e := s.findAgentReview(ctx, Scope{Org: i.TenantID, Actor: i.UserID}, runID)
	if e != nil || !found {
		return TaskStateMetadata{}, found, e
	}
	return r.TaskStateMetadata(), true, nil
}
