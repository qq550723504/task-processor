// Package preparation owns fixed source membership and per-target progress.
// Product bodies, approved assets, credentials and external execution outcomes
// remain facts of their existing owners.
package preparation

import (
	"context"
	"errors"
	"task-processor/internal/product/collection"
	"time"
)

const (
	PermissionRead   = "workbench.supply.read"
	PermissionManage = "workbench.supply.manage"
	PermissionSubmit = "workbench.listing.submit"
	Timeout          = 10 * time.Second
)

var (
	ErrInvalid     = errors.New("invalid preparation request")
	ErrForbidden   = errors.New("preparation permission denied")
	ErrNotFound    = errors.New("preparation not found")
	ErrConflict    = errors.New("preparation operation or revision conflict")
	ErrUnknown     = errors.New("preparation commit outcome unknown")
	ErrUnavailable = errors.New("preparation dependency unavailable")
)

type Scope = collection.Scope
type Query = collection.Query
type TransferInput = collection.BatchSelectionInput

type Preparation struct {
	ID             string    `json:"id"`
	SourceBatchID  string    `json:"sourceBatchId"`
	SourceRevision int64     `json:"sourceRevision"`
	Name           string    `json:"name"`
	Count          int64     `json:"count"`
	Revision       int64     `json:"revision"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SourceItem struct {
	ID                 string            `json:"id"`
	PreparationID      string            `json:"preparationId"`
	CollectionItemID   string            `json:"collectionItemId"`
	CollectionRevision int64             `json:"collectionRevision"`
	Source             collection.Source `json:"source"`
}

type TransferReceipt struct {
	Preparation Preparation `json:"preparation"`
	Replayed    bool        `json:"replayed"`
}

// Commit can only be minted by the live transfer use case. The original
// command is stored for exact replay before consulting mutable collections.
type TransferCommit struct {
	scope Scope
	key   string
	input TransferInput
	proof collection.AuthorizedBatchSelection
}

func (c TransferCommit) Read(ctx context.Context) (Scope, string, TransferInput, collection.AuthorizedBatchSelection, error) {
	scope, input, err := c.proof.Read(ctx)
	if err != nil || scope != c.scope || !collection.ValidID(c.key) || collection.Digest(input) != collection.Digest(c.input) {
		return Scope{}, "", TransferInput{}, collection.AuthorizedBatchSelection{}, ErrForbidden
	}
	return c.scope, c.key, input, c.proof, nil
}

type Authorizer interface {
	Authorize(context.Context, string) (Scope, error)
}
type CollectionSelections interface {
	SelectBatch(context.Context, collection.BatchSelectionInput) (collection.AuthorizedBatchSelection, error)
}
type Repository interface {
	Transfer(context.Context, TransferCommit) (TransferReceipt, error)
	FindTransfer(context.Context, Scope, string, TransferInput) (TransferReceipt, error)
	ReadByKey(context.Context, Scope, string) (TransferReceipt, error)
	List(context.Context, Scope, Query) (collection.Page[Preparation], error)
	ListSources(context.Context, Scope, string, Query) (collection.Page[SourceItem], error)
}

func OperationID(scope Scope, key string) string {
	return collection.StableID(scope.OrganizationID, scope.ActorID, "supply-transfer", key)
}
