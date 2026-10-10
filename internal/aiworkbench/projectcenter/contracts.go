// Package projectcenter owns private long-term context, never execution or referenced facts.
package projectcenter

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid     = errors.New("INVALID_REQUEST")
	ErrForbidden   = errors.New("FORBIDDEN")
	ErrNotFound    = errors.New("NOT_FOUND")
	ErrUnavailable = errors.New("DEPENDENCY_UNAVAILABLE")
	ErrConflict    = errors.New("IDEMPOTENCY_CONFLICT")
	ErrRevision    = errors.New("REVISION_MISMATCH")
	ErrArchived    = errors.New("PROJECT_ARCHIVED")
)

const PermissionRead = "workbench.project.read"
const PermissionManage = "workbench.project.manage"

type Scope struct{ OrganizationID, ActorID string }
type Fields struct {
	Title   string `json:"title"`
	Goal    string `json:"goal"`
	Kind    string `json:"kind"`
	DueDate string `json:"dueDate"`
}
type Project struct {
	ID    string `json:"id"`
	Scope Scope  `json:"-"`
	Fields
	StoreID   string    `json:"-"`
	Archived  bool      `json:"archived"`
	Revision  uint64    `json:"revision"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Reference struct {
	SlotID   string `json:"slotId"`
	Kind     string `json:"kind"`
	TargetID string `json:"targetId,omitempty"`
}
type ReferenceView struct {
	SlotID     string `json:"slotId"`
	Kind       string `json:"kind"`
	Available  bool   `json:"available"`
	TargetID   string `json:"targetId,omitempty"`
	Title      string `json:"title,omitempty"`
	Href       string `json:"href,omitempty"`
	TaskState  string `json:"taskState,omitempty"`
	ResultHref string `json:"resultHref,omitempty"`
}
type View struct {
	Project
	StoreScope           bool            `json:"storeScope"`
	Store                *ReferenceView  `json:"store,omitempty"`
	References           []ReferenceView `json:"references"`
	TaskTotal            int             `json:"taskTotal"`
	TaskCompleted        int             `json:"taskCompleted"`
	TaskPending          int             `json:"taskPending"`
	TaskSummaryAvailable bool            `json:"taskSummaryAvailable"`
}
type Template struct {
	ID    string `json:"id"`
	Scope Scope  `json:"-"`
	Name  string `json:"name"`
	Fields
	Revision  uint64    `json:"revision"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
}
type Command struct {
	Operation string     `json:"operation"`
	ID        string     `json:"id,omitempty"`
	Expected  uint64     `json:"expected,omitempty"`
	Fields    *Fields    `json:"fields,omitempty"`
	StoreID   *string    `json:"storeId,omitempty"`
	Reference *Reference `json:"reference,omitempty"`
	SlotID    string     `json:"slotId,omitempty"`
	Name      string     `json:"name,omitempty"`
}
type Receipt struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
	Replayed bool   `json:"replayed"`
}
type Query struct{ Mode, Search, Kind, WorkScope, StoreID, After string }
type Page struct {
	Projects []View `json:"projects"`
	Next     string `json:"next"`
}
type TemplatePage struct {
	Templates []Template `json:"templates"`
	Next      string     `json:"next"`
}
type Repository interface {
	Replay(context.Context, Scope, string, Command) (Receipt, bool, error)
	Commit(context.Context, Scope, string, Command, error) (Receipt, error)
	Get(context.Context, Scope, string) (Project, []Reference, error)
	List(context.Context, Scope, Query) ([]Project, string, error)
	Templates(context.Context, Scope, string) ([]Template, string, error)
}
type ReferenceReader interface {
	Resolve(context.Context, Scope, Reference) (ReferenceView, error)
}
type Service struct {
	Store     Repository
	Reader    ReferenceReader
	Authorize func(context.Context, Scope, bool) error
}
