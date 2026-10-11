package supplymarket

import (
	"context"
	"encoding/json"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
)

type Authorizer interface {
	Authorize(context.Context, string) (collection.Scope, error)
	AuthorizePlatform(context.Context) (string, error)
}
type SelectionReader interface {
	Select(context.Context, collection.SelectionInput) (collection.AuthorizedSelection, error)
}
type EffectiveProductReader interface {
	ReadEffective(context.Context, collection.Scope, collection.ItemDetail, uint64, string) (catalog.PublishedSnapshot, error)
}
type SelectionInput struct {
	ItemID                string `json:"itemId"`
	ExpectedRevision      int64  `json:"expectedRevision"`
	OriginalPublicationID string `json:"originalPublicationId"`
	OriginalVersion       uint64 `json:"originalVersion,string"`
	EffectiveVersion      uint64 `json:"effectiveVersion,string"`
	ApplyID               string `json:"applyId,omitempty"`
}
type Mutation struct {
	Action               string             `json:"action"`
	ID                   string             `json:"id,omitempty"`
	ExpectedRevision     int64              `json:"expectedRevision,omitempty"`
	Selection            *SelectionInput    `json:"selection,omitempty"`
	Supply               *SupplyDeclaration `json:"supply,omitempty"`
	Connection           *ConnectionInput   `json:"connection,omitempty"`
	FileIDs              []string           `json:"fileIds,omitempty"`
	Disclosure           bool               `json:"disclosure,omitempty"`
	Note                 string             `json:"note,omitempty"`
	CooperationConfirmed bool               `json:"cooperationConfirmed,omitempty"`
}
type SelectedProduct struct {
	Source  SourceReference
	Product PublicProduct
}
type ProductChoice struct {
	Selection SelectionInput `json:"selection"`
	Product   PublicProduct  `json:"product"`
	Optimized bool           `json:"optimized"`
}
type ChoiceReader interface {
	ReadChoice(context.Context, collection.Scope, collection.ItemDetail) (ProductChoice, error)
}

// Choice discovers an exact eligible version after consuming the member's
// sealed Collection selection. It is not a disclosure or mutation authority.
func (s *Service) Choice(ctx context.Context, id string) (ProductChoice, error) {
	if ctx == nil || s == nil {
		return ProductChoice{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) {
		return ProductChoice{}, ErrInvalid
	}
	scope, err := s.auth.Authorize(ctx, PermissionApply)
	if err != nil || scope.Validate() != nil {
		return ProductChoice{}, ErrForbidden
	}
	current, err := s.auth.Authorize(ctx, collection.PermissionManage)
	if err != nil || current != scope {
		return ProductChoice{}, ErrForbidden
	}
	items, ok := s.selected.(interface {
		ReadItem(context.Context, string) (collection.ItemDetail, error)
	})
	choice, ready := s.effective.(ChoiceReader)
	if !ok || !ready {
		return ProductChoice{}, ErrUnavailable
	}
	item, err := items.ReadItem(ctx, id)
	if err != nil {
		return ProductChoice{}, err
	}
	proof, err := s.selected.Select(ctx, collection.SelectionInput{ItemID: id, ExpectedRevision: item.Item.Revision, OriginalPublicationID: item.Item.Source.PublicationID, OriginalVersion: item.Item.Source.Version})
	if err != nil {
		return ProductChoice{}, err
	}
	original, err := proof.Scope(ctx)
	if err != nil || original != scope {
		return ProductChoice{}, ErrForbidden
	}
	item, err = proof.Read(ctx)
	if err != nil || item.Item.Source.Kind != "own" {
		return ProductChoice{}, ErrConflict
	}
	result, err := choice.ReadChoice(ctx, scope, item)
	if err != nil {
		return ProductChoice{}, err
	}
	current, err = s.auth.Authorize(ctx, PermissionApply)
	if err != nil || current != scope {
		return ProductChoice{}, ErrForbidden
	}
	current, err = s.auth.Authorize(ctx, collection.PermissionManage)
	if err != nil || current != scope {
		return ProductChoice{}, ErrForbidden
	}
	return result, ctx.Err()
}

type Command struct {
	Scope                       collection.Scope
	PlatformActor               string
	Key, OperationID, InputHash string
	Input                       Mutation
	// Select is request-local authority. Persistence consumes it only for a new
	// command after locking the exact Collection reference; replay reads receipts.
	Select func(context.Context) (SelectedProduct, error)
}

// Guard is rechecked by persistence immediately before its owner transaction
// commits. It grants no Product reference and cannot replace source disclosure.
type Guard func(context.Context) error
type Repository interface {
	CountApplications(context.Context, collection.Scope) (ApplicationCounts, error)
	Execute(context.Context, Command, Guard) (Receipt, error)
	ReadOperation(context.Context, collection.Scope, string, string) (Receipt, error)
	ListRecords(context.Context, collection.Scope, string, Query) (Page[Record], error)
	ReadRecord(context.Context, collection.Scope, string, string) (Record, error)
	ListEvents(context.Context, collection.Scope, string, string, Query) (Page[Event], error)
	ListRecordReleases(context.Context, collection.Scope, string, string, Query) (Page[Release], error)
	ListReleases(context.Context, Query) (Page[Release], error)
	ReadRelease(context.Context, string) (Release, error)
}
type Service struct {
	auth       Authorizer
	selected   SelectionReader
	effective  EffectiveProductReader
	repository Repository
}

func NewService(auth Authorizer, selected SelectionReader, effective EffectiveProductReader, repository Repository) (*Service, error) {
	if auth == nil || selected == nil || effective == nil || repository == nil {
		return nil, ErrUnavailable
	}
	return &Service{auth, selected, effective, repository}, nil
}

func (s *Service) ExecuteMember(ctx context.Context, key string, input Mutation) (Receipt, error) {
	if ctx == nil || s == nil {
		return Receipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(key) || validateMutation(input, false) != nil {
		return Receipt{}, ErrInvalid
	}
	permission := PermissionApply
	if input.Action == "select_release" {
		permission = PermissionSelect
	}
	scope, err := s.auth.Authorize(ctx, permission)
	if err != nil || scope.Validate() != nil {
		return Receipt{}, ErrForbidden
	}
	if input.Action == "select_release" || input.Action == "create_official_draft" || input.Action == "submit_selected" {
		collectionScope, err := s.auth.Authorize(ctx, collection.PermissionManage)
		if err != nil || collectionScope != scope {
			return Receipt{}, ErrForbidden
		}
	}
	command := Command{Scope: scope, Key: key, OperationID: collection.StableID(scope.OrganizationID, scope.ActorID, key), InputHash: collection.Digest(input), Input: input}
	if input.Selection != nil {
		command.Select = func(ctx context.Context) (SelectedProduct, error) {
			return s.selectOwn(ctx, scope, *input.Selection, input.Action == "submit_selected")
		}
	}
	guard := func(ctx context.Context) error {
		current, err := s.auth.Authorize(ctx, permission)
		if err != nil || current != scope {
			return ErrForbidden
		}
		if input.Action == "select_release" || input.Selection != nil {
			current, err = s.auth.Authorize(ctx, collection.PermissionManage)
			if err != nil || current != scope {
				return ErrForbidden
			}
		}
		return ctx.Err()
	}
	return s.repository.Execute(ctx, command, guard)
}
func (s *Service) ExecutePlatform(ctx context.Context, key string, input Mutation) (Receipt, error) {
	if ctx == nil || s == nil {
		return Receipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(key) || validateMutation(input, true) != nil {
		return Receipt{}, ErrInvalid
	}
	actor, err := s.auth.AuthorizePlatform(ctx)
	if err != nil || !authidentity.IsBoundedIdentifier(actor) {
		return Receipt{}, ErrForbidden
	}
	command := Command{PlatformActor: actor, Key: key, OperationID: collection.StableID("supply-platform", actor, key), InputHash: collection.Digest(input), Input: input}
	guard := func(ctx context.Context) error {
		current, err := s.auth.AuthorizePlatform(ctx)
		if err != nil || current != actor {
			return ErrForbidden
		}
		return ctx.Err()
	}
	return s.repository.Execute(ctx, command, guard)
}

func (s *Service) selectOwn(ctx context.Context, scope collection.Scope, input SelectionInput, requireApply bool) (SelectedProduct, error) {
	proof, err := s.selected.Select(ctx, collection.SelectionInput{ItemID: input.ItemID, ExpectedRevision: input.ExpectedRevision, OriginalPublicationID: input.OriginalPublicationID, OriginalVersion: input.OriginalVersion})
	if err != nil {
		return SelectedProduct{}, err
	}
	owner, err := proof.Scope(ctx)
	if err != nil || owner != scope {
		return SelectedProduct{}, ErrForbidden
	}
	item, err := proof.Read(ctx)
	if err != nil {
		return SelectedProduct{}, err
	}
	if item.Item.Source.Kind != "own" || requireApply && (!collection.ValidID(input.ApplyID) || input.EffectiveVersion <= input.OriginalVersion) {
		return SelectedProduct{}, ErrInvalid
	}
	published, err := s.effective.ReadEffective(ctx, scope, item, input.EffectiveVersion, input.ApplyID)
	if err != nil {
		return SelectedProduct{}, err
	}
	if published.Identity.TenantID != scope.OrganizationID || published.Identity.ProductKey != item.Item.Source.ProductKey || published.Version != input.EffectiveVersion || published.Version == 0 || published.PublicationID == "" {
		return SelectedProduct{}, ErrConflict
	}
	product, err := PublicProjection(published.Snapshot)
	if err != nil {
		return SelectedProduct{}, err
	}
	return SelectedProduct{Source: SourceReference{Scope: scope, ItemID: item.Item.ID, ItemRevision: item.Item.Revision, ProductKey: item.Item.Source.ProductKey, OriginalPublicationID: item.Item.Source.PublicationID, OriginalVersion: item.Item.Source.Version, PublicationID: published.PublicationID, Version: published.Version, ApplyID: input.ApplyID}, Product: product}, nil
}

func validateMutation(i Mutation, platform bool) error {
	raw, err := json.Marshal(i)
	if err != nil || len(raw) > 64<<10 || len(i.FileIDs) > 6 || !text(i.Note, 0, 2000) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range i.FileIDs {
		if !collection.ValidID(id) || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	if platform {
		if !collection.ValidID(i.ID) || i.ExpectedRevision < 1 || i.Selection != nil || i.Supply != nil || i.Connection != nil || len(i.FileIDs) > 0 || i.Disclosure {
			return ErrInvalid
		}
		switch i.Action {
		case "evaluate", "request_supplement", "approve", "reject", "confirm_plan", "close":
			if !text(i.Note, 1, 2000) {
				return ErrInvalid
			}
		case "publish", "revoke":
			if i.CooperationConfirmed {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		return nil
	}
	if i.CooperationConfirmed {
		return ErrInvalid
	}
	switch i.Action {
	case "create_official_draft", "submit_selected":
		if i.ID != "" || i.ExpectedRevision != 0 || i.Selection == nil || i.Supply == nil || i.Supply.Validate() != nil || i.Connection != nil || !i.Disclosure {
			return ErrInvalid
		}
		if i.Action == "submit_selected" && len(i.FileIDs) == 0 {
			return ErrInvalid
		}
		if i.Action == "create_official_draft" && len(i.FileIDs) > 0 {
			return ErrInvalid
		}
	case "submit_connection":
		if i.ID != "" || i.ExpectedRevision != 0 || i.Selection != nil || i.Supply != nil || i.Connection == nil || i.Connection.Validate() != nil || len(i.FileIDs) > 0 || i.Disclosure {
			return ErrInvalid
		}
	case "supplement":
		if !collection.ValidID(i.ID) || i.ExpectedRevision < 1 || i.Selection != nil || i.Supply != nil || i.Connection != nil || i.Disclosure || !text(i.Note, 1, 2000) {
			return ErrInvalid
		}
	case "select_release":
		if !collection.ValidID(i.ID) || i.ExpectedRevision < 1 || i.Selection != nil || i.Supply != nil || i.Connection != nil || len(i.FileIDs) > 0 || i.Disclosure || i.Note != "" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (s *Service) ListMarket(ctx context.Context, q Query) (Page[Release], error) {
	if ctx == nil || s == nil {
		return Page[Release]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if q.Validate() != nil || q.Kind == "connection" {
		return Page[Release]{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, false)
	if err != nil {
		return Page[Release]{}, ErrForbidden
	}
	value, err := s.repository.ListReleases(ctx, q)
	if err != nil {
		return Page[Release]{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, false); err != nil {
		return Page[Release]{}, err
	}
	return value, nil
}
func (s *Service) ReadMarket(ctx context.Context, id string) (Release, error) {
	if ctx == nil || s == nil {
		return Release{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) {
		return Release{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, false)
	if err != nil {
		return Release{}, ErrForbidden
	}
	value, err := s.repository.ReadRelease(ctx, id)
	if err != nil {
		return Release{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, false); err != nil {
		return Release{}, err
	}
	return value, nil
}
func (s *Service) ListApplications(ctx context.Context, q Query, platform bool) (Page[Record], error) {
	if ctx == nil || s == nil {
		return Page[Record]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if q.Validate() != nil {
		return Page[Record]{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Page[Record]{}, err
	}
	value, err := s.repository.ListRecords(ctx, scope, actor, q)
	if err != nil {
		return Page[Record]{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Page[Record]{}, err
	}
	return value, nil
}
func (s *Service) ReadApplication(ctx context.Context, id string, platform bool) (Record, error) {
	if ctx == nil || s == nil {
		return Record{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) {
		return Record{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Record{}, err
	}
	value, err := s.repository.ReadRecord(ctx, scope, actor, id)
	if err != nil {
		return Record{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Record{}, err
	}
	return value, nil
}
func (s *Service) Events(ctx context.Context, id string, q Query, platform bool) (Page[Event], error) {
	if ctx == nil || s == nil {
		return Page[Event]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) || q.Validate() != nil {
		return Page[Event]{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Page[Event]{}, err
	}
	value, err := s.repository.ListEvents(ctx, scope, actor, id, q)
	if err != nil {
		return Page[Event]{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Page[Event]{}, err
	}
	return value, nil
}
func (s *Service) Operation(ctx context.Context, id string, platform bool) (Receipt, error) {
	if ctx == nil || s == nil {
		return Receipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) {
		return Receipt{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Receipt{}, err
	}
	value, err := s.repository.ReadOperation(ctx, scope, actor, id)
	if err != nil {
		return Receipt{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Receipt{}, err
	}
	return value, nil
}
func (s *Service) RecordReleases(ctx context.Context, id string, q Query, platform bool) (Page[Release], error) {
	if ctx == nil || s == nil {
		return Page[Release]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(id) || q.Validate() != nil {
		return Page[Release]{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Page[Release]{}, err
	}
	result, err := s.repository.ListRecordReleases(ctx, scope, actor, id, q)
	if err != nil {
		return Page[Release]{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Page[Release]{}, err
	}
	return result, nil
}
func (s *Service) ByKey(ctx context.Context, key string, platform bool) (Receipt, error) {
	if ctx == nil || s == nil {
		return Receipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !collection.ValidID(key) {
		return Receipt{}, ErrInvalid
	}
	scope, actor, err := s.readPrincipal(ctx, platform)
	if err != nil {
		return Receipt{}, err
	}
	id := collection.StableID(scope.OrganizationID, scope.ActorID, key)
	if platform {
		id = collection.StableID("supply-platform", actor, key)
	}
	value, err := s.repository.ReadOperation(ctx, scope, actor, id)
	if err != nil {
		return Receipt{}, err
	}
	if err = s.verifyReadPrincipal(ctx, scope, actor, platform); err != nil {
		return Receipt{}, err
	}
	return value, nil
}
func (s *Service) verifyReadPrincipal(ctx context.Context, scope collection.Scope, actor string, platform bool) error {
	current, publisher, err := s.readPrincipal(ctx, platform)
	if err != nil || current != scope || publisher != actor {
		return ErrForbidden
	}
	return ctx.Err()
}
func (s *Service) readPrincipal(ctx context.Context, platform bool) (collection.Scope, string, error) {
	if ctx == nil || s == nil {
		return collection.Scope{}, "", ErrForbidden
	}
	if platform {
		actor, err := s.auth.AuthorizePlatform(ctx)
		if err != nil || !authidentity.IsBoundedIdentifier(actor) {
			return collection.Scope{}, "", ErrForbidden
		}
		return collection.Scope{}, actor, nil
	}
	scope, err := s.auth.Authorize(ctx, PermissionRead)
	if err != nil || scope.Validate() != nil {
		return collection.Scope{}, "", ErrForbidden
	}
	return scope, "", nil
}
