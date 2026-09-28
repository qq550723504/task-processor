// Package knowledge owns current-enterprise document lifecycle facts.
package knowledge

import (
	"context"
	"errors"
	"time"
)

const MaxUploadBytes = 10 << 20
const MaxTextBytes = 2 << 20
const MaxActiveSources = 4

type State string

const (
	Active    State = "ACTIVE"
	Disabling State = "DISABLING"
	Disabled  State = "DISABLED"
)

type ProcessingState string

const (
	Admitted     ProcessingState = "ADMITTED"
	ObjectStored ProcessingState = "OBJECT_STORED"
	Processing   ProcessingState = "PROCESSING"
	Available    ProcessingState = "AVAILABLE"
	Partial      ProcessingState = "PARTIAL"
	Failed       ProcessingState = "FAILED"
)

var (
	ErrInvalid      = errors.New("KNOWLEDGE_INVALID_REQUEST")
	ErrNotFound     = errors.New("KNOWLEDGE_NOT_FOUND")
	ErrConflict     = errors.New("KNOWLEDGE_CONFLICT")
	ErrInactive     = errors.New("KNOWLEDGE_DISABLED")
	ErrSourceLimit  = errors.New("KNOWLEDGE_SOURCE_LIMIT_REACHED")
	ErrRevisionBusy = errors.New("KNOWLEDGE_REVISION_IN_PROGRESS")
	ErrNotReadable  = errors.New("KNOWLEDGE_NOT_READY")
	ErrIntegrity    = errors.New("KNOWLEDGE_INTEGRITY_FAILURE")
	ErrUnavailable  = errors.New("KNOWLEDGE_UNAVAILABLE")
	ErrLeaseLost    = errors.New("KNOWLEDGE_LEASE_LOST")
)

type Scope struct{ OrganizationID, ActorID string }
type Base struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"-"`
	Name           string    `json:"name"`
	State          State     `json:"state"`
	Version        int64     `json:"version"`
	FenceVersion   int64     `json:"-"`
	CreatedBy      string    `json:"createdBy"`
	UpdatedBy      string    `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type Revision struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"-"`
	SourceID       string          `json:"-"`
	Number         int64           `json:"number"`
	Filename       string          `json:"filename"`
	ContentType    string          `json:"contentType"`
	SizeBytes      int64           `json:"sizeBytes"`
	SHA256         string          `json:"-"`
	ObjectKey      string          `json:"-"`
	State          ProcessingState `json:"state"`
	Failure        string          `json:"failure,omitempty"`
	Warning        string          `json:"warning,omitempty"`
	Text           string          `json:"-"`
	LeaseOwner     string          `json:"-"`
	LeaseUntil     *time.Time      `json:"-"`
	Attempts       int             `json:"-"`
	NextAttemptAt  time.Time       `json:"-"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}
type Source struct {
	ID                        string    `json:"id"`
	OrganizationID            string    `json:"-"`
	BaseID                    string    `json:"knowledgeBaseId"`
	BaseState                 State     `json:"-" gorm:"-"`
	Name                      string    `json:"name"`
	State                     State     `json:"state"`
	Version                   int64     `json:"version"`
	FenceVersion              int64     `json:"-"`
	LatestRevisionID          string    `json:"-"`
	CurrentReadableRevisionID string    `json:"-"`
	LatestRevision            *Revision `json:"latestRevision" gorm:"-"`
	CurrentReadableRevision   *Revision `json:"currentReadableRevision" gorm:"-"`
	CreatedBy                 string    `json:"createdBy"`
	UpdatedBy                 string    `json:"updatedBy"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}
type Command struct {
	Scope                                          Scope
	Kind, Key, Fingerprint, BaseID, SourceID, Name string
	Version                                        int64
	Upload                                         *Revision
}
type Result struct {
	Base     *Base     `json:"knowledgeBase,omitempty"`
	Source   *Source   `json:"source,omitempty"`
	Revision *Revision `json:"revision,omitempty"`
}
type Preview struct {
	RevisionID string `json:"revisionId"`
	Text       string `json:"text"`
	Warning    string `json:"warning,omitempty"`
}

// Repository serializes admission and lifecycle writes in its own database.
// The implementation must never hold a transaction over object/parser I/O.
type Repository interface {
	Apply(context.Context, Command) (Result, error)
	ListBases(context.Context, string, int, int) ([]Base, int64, error)
	GetBase(context.Context, string, string) (Base, error)
	ListSources(context.Context, string, string) ([]Source, error)
	GetSource(context.Context, string, string) (Source, error)
	Preview(context.Context, string, string, string) (Preview, error)
	ClaimUpload(context.Context, string, string, string) (Revision, bool, error)
	ClaimProcessing(context.Context, string, int) ([]Revision, error)
	ConfirmObject(context.Context, Revision) error
	Finish(context.Context, Revision, ParseResult) error
}
type Object struct {
	Key, SHA256, ContentType string
	SizeBytes                int64
	Data                     []byte
}
type Inspection struct {
	Exists    bool
	SHA256    string
	SizeBytes int64
}
type KnowledgeObjectStore interface {
	PutImmutable(context.Context, Object) error
	Inspect(context.Context, Object) (Inspection, error)
	ReadBounded(context.Context, Object, int64) ([]byte, error)
}
type ParseResult struct {
	Text, Warning, Failure string
	Transient              bool
}
type DocumentParser interface {
	Parse(context.Context, string, []byte) ParseResult
}
