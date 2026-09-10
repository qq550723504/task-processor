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

// SourcePublicationGateway performs the root, freshly authorized SRC-1 read
// and can mint the private request proof consumed by a transaction-bound read.
type SourcePublicationGateway interface {
	SourcePublicationReader
	AuthorizeRead(context.Context) (context.Context, error)
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
	SourceReader() SourcePublicationReader
}
type Store interface {
	Read(context.Context, Scope, string) (Record, error)
	List(context.Context, Scope, PageRequest) (Page, error)
	Preflight(context.Context, Operation) (View, bool, error)
	Run(context.Context, Operation, func(Tx) (View, error)) (View, error)
}
