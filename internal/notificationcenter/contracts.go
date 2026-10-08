// Package notificationcenter owns announcements and reading receipts. Business
// facts and authorization remain with the source owners.
package notificationcenter

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid           = errors.New("invalid notification request")
	ErrForbidden         = errors.New("notification permission denied")
	ErrNotFound          = errors.New("notification not found")
	ErrUnavailable       = errors.New("notification source unavailable")
	ErrDependencyMissing = errors.New("notification engine not opened")
	ErrConflict          = errors.New("notification idempotency conflict")
	ErrStale             = errors.New("notification snapshot stale")
	ErrCapacity          = errors.New("notification capacity exceeded")
)

const (
	Official           = "official"
	Business           = "business"
	SchemaVersion      = "notification-center-v1"
	MaxItems           = 10000
	MaxCollectionBytes = 8 << 20
	SnapshotTTL        = 5 * time.Minute
)

// Scope is constructed from verified server identity, never a request body.
type Scope struct {
	Realm          string
	Subject        string
	OrganizationID string
	Category       string
}
type Ref struct {
	Source         string `json:"source"`
	EntityID       string `json:"entityId"`
	Type           string `json:"type"`
	Revision       string `json:"revision"`
	OrganizationID string `json:"organizationId"`
}
type Target struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Item struct {
	Ref            Ref        `json:"-"`
	ID             string     `json:"id"`
	Category       string     `json:"category"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Paragraphs     []string   `json:"paragraphs"`
	OccurredAt     *time.Time `json:"occurredAt"`
	Source         string     `json:"source"`
	OrganizationID string     `json:"organizationId"`
	Attention      bool       `json:"attention"`
	Read           bool       `json:"read"`
	Target         Target     `json:"target"`
	Href           string     `json:"href"`
	// Association is an internal exact run binding used only after both items
	// have passed their original source authorization. It is never serialized.
	Association string `json:"-"`
}
type SourcePage struct {
	Items []Item
	Next  string
}
type Source interface {
	Name() string
	Category() string
	Personal() bool
	ListVisible(context.Context, Scope, string, int) (SourcePage, error)
	ReadVisible(context.Context, Scope, Ref) (Item, error)
}

// ResourceBatchReader performs the same fresh authorization as ReadVisible,
// in one bounded owner enumeration for the original snapshot references.
type ResourceBatchReader interface {
	ReadVisibleRefs(context.Context, Scope, []Ref) ([]Item, error)
}
type Coverage struct {
	Source     string    `json:"source"`
	State      string    `json:"state"`
	Complete   bool      `json:"complete"`
	ObservedAt time.Time `json:"observedAt"`
}
type ListRequest struct {
	Filter, After string
	Limit         int
}
type List struct {
	SchemaVersion string     `json:"schemaVersion"`
	Items         []Item     `json:"items"`
	Coverage      []Coverage `json:"coverage"`
	Exact         bool       `json:"exact"`
	Count         int        `json:"count"`
	Unread        int        `json:"unread"`
	Pending       int        `json:"pending"`
	Next          string     `json:"next"`
}
type Command struct {
	Key         string    `json:"key"`
	Operation   string    `json:"operation"`
	Fingerprint string    `json:"-"`
	CommittedAt time.Time `json:"committedAt"`
	ResultID    string    `json:"resultId,omitempty"`
}
type Snapshot struct {
	ID          string    `json:"id"`
	Fingerprint string    `json:"fingerprint"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Count       int       `json:"count"`
	Refs        []Ref     `json:"-"`
}
type AnnouncementInput struct {
	Category   string   `json:"category"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Paragraphs []string `json:"paragraphs"`
	Target     Target   `json:"target"`
}

// Repository implementations commit receipts and their command atomically.
// No business-owner transaction or external effect is part of these methods.
type Repository interface {
	ReadStates(context.Context, Scope, []Ref) (map[string]bool, error)
	Replay(context.Context, Scope, string, string, string) (Command, bool, error)
	Command(context.Context, Scope, string) (Command, error)
	CommitRead(context.Context, Scope, Command, []Ref, string) (Command, error)
	CreateSnapshot(context.Context, Scope, Command, Snapshot) (Snapshot, error)
	Snapshot(context.Context, Scope, string) (Snapshot, error)
	Publish(context.Context, Scope, Command, AnnouncementInput) (Command, error)
	Withdraw(context.Context, Scope, Command, string, int64) (Command, error)
}
type Service struct {
	Repository Repository
	Sources    []Source
	Now        func() time.Time
}
