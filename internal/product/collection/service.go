package collection

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type Service struct {
	store        Repository
	auth         Authorizer
	acquisitions sourcing.PublishedAcquisitionReader
	snapshots    catalog.VersionedSnapshotReader
	media        SourceMedia
}

func NewService(store Repository, auth Authorizer, acquisitions sourcing.PublishedAcquisitionReader) (*Service, error) {
	if store == nil || auth == nil || acquisitions == nil {
		return nil, ErrUnavailable
	}
	return &Service{store: store, auth: auth, acquisitions: acquisitions}, nil
}
func (s *Service) WithSnapshots(reader catalog.VersionedSnapshotReader) *Service {
	s.snapshots = reader
	return s
}
func (s *Service) authorize(ctx context.Context, permission string) (Scope, error) {
	if err := ctx.Err(); err != nil {
		return Scope{}, err
	}
	scope, err := s.auth.Authorize(ctx, permission)
	if err != nil {
		return Scope{}, err
	}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}
func (s *Service) Mutate(ctx context.Context, key string, input Mutation) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionManage)
	if err != nil {
		return Receipt{}, err
	}
	if !ValidID(key) || !validMutationShape(input) {
		return Receipt{}, ErrInvalid
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > MaxPayloadBytes {
		return Receipt{}, ErrInvalid
	}
	command := Command{Scope: scope, Key: key, OperationID: StableID(scope.OrganizationID, scope.ActorID, key), InputHash: Digest(input), Mutation: input}
	for _, id := range []string{input.BatchID, input.ItemID, input.TargetBatchID} {
		if id != "" && !ValidID(id) {
			return Receipt{}, ErrInvalid
		}
	}
	switch input.Action {
	case "import_products":
		if !validName(input.Name) || len(input.Products) < 1 || len(input.Products) > 200 {
			return Receipt{}, ErrInvalid
		}
		for index, product := range input.Products {
			envelope, err := OwnEnvelope(StableID(command.OperationID, "row", strconv.Itoa(index)), product)
			if err != nil {
				return Receipt{}, err
			}
			command.Envelopes = append(command.Envelopes, envelope)
		}
	case "create_batch":
		if !validName(input.Name) || input.BatchID != "" || input.ItemID != "" || input.Product != nil || input.SourceOperationID != "" {
			return Receipt{}, ErrInvalid
		}
	case "rename_batch", "archive_batch":
		if input.BatchID == "" || input.ExpectedRevision <= 0 || input.Action == "rename_batch" && !validName(input.Name) {
			return Receipt{}, ErrInvalid
		}
	case "move_item", "archive_item":
		if input.ItemID == "" || input.ExpectedRevision <= 0 || input.Action == "move_item" && input.TargetBatchID == "" {
			return Receipt{}, ErrInvalid
		}
	case "add_acquisition":
		if !ValidID(input.SourceOperationID) || input.Product != nil {
			return Receipt{}, ErrInvalid
		}
		published, err := s.acquisitions.ReadPublished(ctx, input.SourceOperationID)
		if err != nil {
			return Receipt{}, err
		}
		op, receipt := published.Result.Operation, published.Result.Publication
		if op.ID != input.SourceOperationID || op.Scope.OrganizationID != scope.OrganizationID || op.Scope.ActorID != scope.ActorID || op.State != sourcing.AcquisitionPublished || receipt == nil {
			return Receipt{}, ErrNotFound
		}
		ref := receipt.Receipt
		if ref.OrganizationID != scope.OrganizationID || ref.ActorID != scope.ActorID || ref.ProductKey != published.Snapshot.Identity.ProductKey || published.Snapshot.Identity.TenantID != scope.OrganizationID || ref.CatalogVersion != published.Snapshot.Version || ref.CatalogPublicationID != published.Snapshot.PublicationID {
			return Receipt{}, ErrNotFound
		}
		command.Source = &Source{ProductKey: ref.ProductKey, PublicationID: ref.PublicationID, Version: ref.CatalogVersion, OperationID: op.ID, Kind: "acquisition"}
	case "create_product":
		if input.Product == nil || input.SourceOperationID != "" {
			return Receipt{}, ErrInvalid
		}
		envelope, err := OwnEnvelope(command.OperationID, *input.Product)
		if err != nil {
			return Receipt{}, err
		}
		command.Envelope = &envelope
	default:
		return Receipt{}, ErrInvalid
	}
	return s.store.Execute(ctx, command)
}
func validName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && utf8.ValidString(name) && len([]byte(name)) <= 200 && !strings.ContainsAny(name, "\x00\r\n")
}

// Each command accepts only its own fields, even for direct domain callers.
func validMutationShape(input Mutation) bool {
	allowed := Mutation{Action: input.Action}
	switch input.Action {
	case "import_products":
		allowed.Name, allowed.Products = input.Name, input.Products
	case "create_batch":
		allowed.Name = input.Name
	case "rename_batch":
		allowed.BatchID, allowed.ExpectedRevision, allowed.Name = input.BatchID, input.ExpectedRevision, input.Name
	case "archive_batch":
		allowed.BatchID, allowed.ExpectedRevision = input.BatchID, input.ExpectedRevision
	case "move_item":
		allowed.ItemID, allowed.TargetBatchID, allowed.ExpectedRevision = input.ItemID, input.TargetBatchID, input.ExpectedRevision
	case "archive_item":
		allowed.ItemID, allowed.ExpectedRevision = input.ItemID, input.ExpectedRevision
	case "add_acquisition":
		allowed.BatchID, allowed.SourceOperationID = input.BatchID, input.SourceOperationID
	case "create_product":
		allowed.BatchID, allowed.Product = input.BatchID, input.Product
	default:
		return false
	}
	return Digest(input) == Digest(allowed)
}
func (s *Service) ListBatches(ctx context.Context, query Query) (Page[Batch], error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return Page[Batch]{}, err
	}
	if err := query.Validate(); err != nil {
		return Page[Batch]{}, err
	}
	return s.store.ListBatches(ctx, scope, query)
}

// ReadBatch consumes the existing owner-scoped repository read; it does not
// widen private Collection access for aggregate consumers.
func (s *Service) ReadBatch(ctx context.Context, id string) (Batch, error) {
	if ctx == nil || s == nil {
		return Batch{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return Batch{}, err
	}
	if !ValidID(id) {
		return Batch{}, ErrInvalid
	}
	batch, err := s.store.ReadBatch(ctx, scope, id)
	if err != nil {
		return Batch{}, err
	}
	current, err := s.authorize(ctx, PermissionRead)
	if err != nil || current != scope {
		return Batch{}, ErrForbidden
	}
	return batch, ctx.Err()
}
func (s *Service) ListItems(ctx context.Context, batchID string, query Query) (Page[Item], error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return Page[Item]{}, err
	}
	if batchID != "" && !ValidID(batchID) || query.Validate() != nil {
		return Page[Item]{}, ErrInvalid
	}
	if batchID != "" {
		if _, err := s.store.ReadBatch(ctx, scope, batchID); err != nil {
			return Page[Item]{}, err
		}
	}
	return s.store.ListItems(ctx, scope, batchID, query)
}
func (s *Service) ReadItem(ctx context.Context, itemID string) (ItemDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return ItemDetail{}, err
	}
	if !ValidID(itemID) {
		return ItemDetail{}, ErrInvalid
	}
	return s.readItem(ctx, scope, itemID)
}

func (s *Service) readItem(ctx context.Context, scope Scope, itemID string) (ItemDetail, error) {
	item, err := s.store.ReadItem(ctx, scope, itemID)
	if err != nil {
		return ItemDetail{}, err
	}
	if s.snapshots == nil {
		return ItemDetail{}, ErrUnavailable
	}
	snapshot, err := s.snapshots.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Source.ProductKey}, item.Source.Version)
	if err != nil {
		return ItemDetail{}, err
	}
	if snapshot.Identity.TenantID != scope.OrganizationID || snapshot.Identity.ProductKey != item.Source.ProductKey || snapshot.Version != item.Source.Version || snapshot.PublicationID != item.Source.PublicationID {
		return ItemDetail{}, ErrUnavailable
	}
	return ItemDetail{Item: item, Product: snapshot.Snapshot}, nil
}
func (s *Service) ReadOperation(ctx context.Context, key string) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return Receipt{}, err
	}
	if !ValidID(key) {
		return Receipt{}, ErrInvalid
	}
	return s.store.ReadOperation(ctx, scope, StableID(scope.OrganizationID, scope.ActorID, key))
}
