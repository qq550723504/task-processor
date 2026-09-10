package review

import (
	"context"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type Scope struct {
	Org, Actor string
	Admin      bool
}
type Authorizer interface {
	Authorize(string, []string, string) bool
	IsTenantAdmin(string, []string) bool
}

// SourcePublicationReader exposes only the exact, freshly authorized SRC-1
// read needed to prove the evidence behind one Catalog publication.
type SourcePublicationReader interface {
	Read(context.Context, string) (sourcing.PersistedPublication, error)
}
type Operation struct {
	Scope            Scope
	Key, Fingerprint string
}
type Tx interface {
	Load(string) (Record, error)
	Replay() (View, bool, error)
	Save(Record) error
	Complete(View) error
	Publisher() *catalog.Publisher
	Reader() catalog.VersionedSnapshotReader
}
type Store interface {
	Read(context.Context, Scope, string) (Record, error)
	List(context.Context, Scope, PageRequest) (Page, error)
	FindOperation(context.Context, Operation) (View, bool, error)
	Run(context.Context, Operation, func(Tx) (View, error)) (View, error)
}
