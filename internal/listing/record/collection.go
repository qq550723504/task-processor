package record

import (
	"context"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"

	"github.com/google/uuid"
)

const MaxPageSize = 100

type CollectionItem struct {
	ID        string
	Input     Input
	CreatedAt time.Time
}

type PageCursor struct {
	ID        string
	CreatedAt time.Time
}

type PageRequest struct {
	Limit  int
	Cursor *PageCursor
}

func (r PageRequest) Validate() error {
	if r.Limit < 1 || r.Limit > MaxPageSize {
		return ErrInvalid
	}
	if r.Cursor == nil {
		return nil
	}
	parsed, err := uuid.Parse(r.Cursor.ID)
	if err != nil || parsed.String() != r.Cursor.ID || r.Cursor.CreatedAt.IsZero() || r.Cursor.CreatedAt.Location() != time.UTC {
		return ErrInvalid
	}
	return nil
}

type Page struct {
	Items      []CollectionItem
	NextCursor *PageCursor
}

type CollectionReader interface {
	List(context.Context, listingtask.Actor, PageRequest) (Page, error)
}

type CollectionService struct {
	reader CollectionReader
	auth   Authorizer
}

func NewCollectionService(reader CollectionReader, auth Authorizer) (*CollectionService, error) {
	if reader == nil || auth == nil {
		return nil, ErrUnavailable
	}
	return &CollectionService{reader: reader, auth: auth}, nil
}

func (s *CollectionService) List(ctx context.Context, request PageRequest) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	actor := listingtask.Actor{TenantID: identity.EffectiveOrganizationID, UserID: identity.UserID, Roles: identity.Roles}
	if !ok || identity.TenantID != actor.TenantID || listingtask.ValidateActor(actor) != nil || !time.Now().Before(identity.TokenExpiresAt) || !s.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminRead) {
		return Page{}, ErrForbidden
	}
	if err := request.Validate(); err != nil {
		return Page{}, err
	}
	page, err := s.reader.List(ctx, actor, request)
	if ctx.Err() != nil {
		return Page{}, ctx.Err()
	}
	return page, err
}
