// Package record owns immutable, locally saved SHEIN draft records.
package record

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/product/catalog"

	"github.com/google/uuid"
)

const Timeout = 5 * time.Second
const MaxPayloadBytes = 2 << 20

var (
	ErrInvalid     = errors.New("invalid listing record request")
	ErrForbidden   = errors.New("listing record permission denied")
	ErrNotFound    = errors.New("listing record input or resource unavailable")
	ErrConflict    = errors.New("listing record operation conflict")
	ErrUnavailable = errors.New("listing record dependency unavailable")
	ErrTooLarge    = errors.New("listing record payload exceeds limit")
)

type Input struct {
	ProductKey      string `json:"product_key"`
	SnapshotVersion uint64 `json:"snapshot_version"`
	Country         string `json:"country"`
	Language        string `json:"language"`
}

func (i Input) Validate() error {
	if i.ProductKey == "" || i.ProductKey != strings.TrimSpace(i.ProductKey) || len(i.ProductKey) > 128 || i.SnapshotVersion == 0 || i.SnapshotVersion > 1<<63-1 || i.Country != "US" || i.Language != "en" {
		return ErrInvalid
	}
	return nil
}

type Record struct {
	ID             string
	OrganizationID string
	OwnerUserID    string
	OperationID    string
	Input          Input
	Payload        []byte
	CreatedAt      time.Time
	ReadAt         time.Time
}

func (r Record) Clone() Record { r.Payload = append([]byte(nil), r.Payload...); return r }

type Receipt struct {
	RecordID string `json:"record_id"`
}

// Prepared can only be populated by the admitted creation use case. There is
// no exported arbitrary-payload constructor or mutable persistence record.
type Prepared struct{ record Record }

func (p Prepared) Record() Record { return p.record.Clone() }

type Reader interface {
	ReadOfflinePackage(context.Context, listingtask.Actor, string) (Record, error)
}
type Store interface {
	FindOperation(context.Context, listingtask.Actor, string) (Record, error)
	Insert(context.Context, Prepared) (Record, error)
}
type Authorizer interface {
	Authorize(string, []string, string) bool
	IsTenantAdmin(string, []string) bool
}
type Builder interface {
	Build(context.Context, catalog.ProductSnapshot, Input) ([]byte, error)
}
type Service struct {
	source  catalog.VersionedSnapshotReader
	store   Store
	builder Builder
	auth    Authorizer
}

func NewService(source catalog.VersionedSnapshotReader, store Store, builder Builder, auth Authorizer) (*Service, error) {
	if source == nil || store == nil || builder == nil || auth == nil {
		return nil, ErrUnavailable
	}
	return &Service{source, store, builder, auth}, nil
}
func (s *Service) Create(ctx context.Context, operation string, input Input) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	actor := listingtask.Actor{TenantID: identity.EffectiveOrganizationID, UserID: identity.UserID, Roles: identity.Roles}
	if !ok || identity.TenantID != actor.TenantID || listingtask.ValidateActor(actor) != nil || !time.Now().Before(identity.TokenExpiresAt) {
		return Receipt{}, ErrForbidden
	}
	if !s.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminRead) || !s.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminWrite) {
		return Receipt{}, ErrForbidden
	}
	if input.Validate() != nil || listingtask.ValidateTaskID(operation) != nil {
		return Receipt{}, ErrInvalid
	}
	existing, lookupErr := s.store.FindOperation(ctx, actor, operation)
	if canceled := ctx.Err(); canceled != nil {
		return Receipt{}, canceled
	}
	if lookupErr != nil && !errors.Is(lookupErr, ErrNotFound) {
		return Receipt{}, lookupErr
	}
	if lookupErr == nil && existing.Input != input {
		return Receipt{}, ErrConflict
	}
	sourceID := catalog.SnapshotIdentity{TenantID: actor.TenantID, ProductKey: input.ProductKey}
	// Even replay reauthorizes and rechecks the exact allowed source version.
	published, err := s.source.GetSnapshot(ctx, sourceID, input.SnapshotVersion)
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	if published.Identity != sourceID || published.Version != input.SnapshotVersion {
		return Receipt{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if lookupErr == nil {
		return receiptFor(existing, input)
	}
	payload, err := s.builder.Build(ctx, published.Snapshot, input)
	if err != nil {
		return Receipt{}, err
	}
	if len(payload) == 0 || len(payload) > MaxPayloadBytes {
		return Receipt{}, ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	created, err := s.store.Insert(ctx, Prepared{record: Record{ID: uuid.NewString(), OrganizationID: actor.TenantID, OwnerUserID: actor.UserID, OperationID: operation, Input: input, Payload: payload}})
	if err != nil {
		return Receipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	return receiptFor(created, input)
}
func receiptFor(r Record, input Input) (Receipt, error) {
	if r.Input != input {
		return Receipt{}, ErrConflict
	}
	return Receipt{RecordID: r.ID}, nil
}
