package target

import (
	"context"
	"encoding/json"
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
	"time"
)

var ErrTargetUnknown = errors.New("listing target command outcome is unknown; read the original command key")

// Target records use the current private source selection. They neither wrap
// the older local draft service nor accept ProductKey as read authority.
type TargetInput struct {
	SourceID         string                   `json:"sourceId"`
	StoreID          string                   `json:"storeId"`
	ExpectedRevision int64                    `json:"expectedRevision"`
	EffectiveVersion uint64                   `json:"effectiveVersion,string"`
	ApplyReceiptID   string                   `json:"applyReceiptId,omitempty"`
	Draft            goods.OfficialDraftInput `json:"draft"`
}
type TargetRecord struct {
	ID               string                             `json:"id"`
	TargetID         string                             `json:"targetId"`
	Revision         int64                              `json:"revision"`
	Source           preparation.SourceItem             `json:"source"`
	EffectiveVersion uint64                             `json:"effectiveVersion,string"`
	ApplyReceiptID   string                             `json:"applyReceiptId,omitempty"`
	ProductHash      string                             `json:"productHash"`
	InventoryHash    string                             `json:"inventoryHash"`
	RulesHash        string                             `json:"rulesHash"`
	Merchant         storecenter.ProductMerchantBinding `json:"merchant"`
	Input            TargetInput                        `json:"input"`
	Result           goods.OfficialDraft                `json:"result"`
	CreatedAt        time.Time                          `json:"createdAt"`
}
type TargetReceipt struct {
	Record   TargetRecord `json:"record"`
	Replayed bool         `json:"replayed"`
}
type TargetPrepared struct {
	scope          collection.Scope
	key, inputHash string
	source         preparation.AuthorizedSource
	record         TargetRecord
}

func (p TargetPrepared) Read(ctx context.Context) (collection.Scope, string, string, TargetRecord, error) {
	scope, source, _, err := p.source.Read(ctx)
	if err != nil || scope != p.scope || source != p.record.Source || !collection.ValidID(p.key) || collection.Digest(p.record.Input) != p.inputHash {
		return collection.Scope{}, "", "", TargetRecord{}, ErrForbidden
	}
	cloned, err := cloneTarget(p.record)
	if err != nil {
		return collection.Scope{}, "", "", TargetRecord{}, ErrUnavailable
	}
	return scope, p.key, p.inputHash, cloned, nil
}
func cloneTarget(input TargetRecord) (TargetRecord, error) {
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > MaxPayloadBytes {
		return TargetRecord{}, ErrTooLarge
	}
	var output TargetRecord
	err = json.Unmarshal(raw, &output)
	return output, err
}

type TargetSourceSelector interface {
	Select(context.Context, string) (preparation.AuthorizedSource, error)
}

type TargetExecutionSourceSelector interface {
	SelectForExecution(context.Context, collection.Scope, string) (preparation.AuthorizedSource, error)
}
type EffectiveTargetProductReader interface {
	ReadEffectiveTargetProduct(context.Context, preparation.AuthorizedSource, uint64, string) (catalog.PublishedSnapshot, error)
}

type ApprovedAssetReader interface {
	GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error)
}
type TargetRuleReader interface {
	ReadTargetRules(context.Context, collection.Scope, string, goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error)
}
type TargetRepository interface {
	FindTargetCommand(context.Context, collection.Scope, string, string) (TargetReceipt, error)
	SaveTarget(context.Context, TargetPrepared) (TargetReceipt, error)
	ReadTargetHead(context.Context, collection.Scope, string) (TargetRecord, error)
	ReadTargetRecord(context.Context, collection.Scope, string) (TargetRecord, error)
	ReadTargetCommand(context.Context, collection.Scope, string) (TargetReceipt, error)
}
type TargetDependencies struct {
	Sources                TargetSourceSelector
	Products               EffectiveTargetProductReader
	Assets                 ApprovedAssetReader
	Rules                  TargetRuleReader
	Records                TargetRepository
	Authorizer             preparation.Authorizer
	ExecutionSources       TargetExecutionSourceSelector
	ExecutionAuthorization collection.ExecutionAuthorizer
}
type TargetService struct{ dependencies TargetDependencies }

func NewTargetService(d TargetDependencies) (*TargetService, error) {
	if d.Sources == nil || d.Products == nil || d.Assets == nil || d.Rules == nil || d.Records == nil || d.Authorizer == nil {
		return nil, ErrUnavailable
	}
	return &TargetService{d}, nil
}
func TargetIdentity(scope collection.Scope, sourceID, storeID string) string {
	return collection.StableID(scope.OrganizationID, scope.ActorID, "supply-target", sourceID, storeID, "shein-us")
}
func (s *TargetService) Create(ctx context.Context, key string, input TargetInput) (TargetReceipt, error) {
	if ctx == nil || s == nil {
		return TargetReceipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorizeTarget(ctx, preparation.PermissionManage)
	if err != nil {
		return TargetReceipt{}, err
	}
	return s.create(ctx, scope, key, input, s.dependencies.Sources.Select, func(ctx context.Context) error {
		current, err := s.authorizeTarget(ctx, preparation.PermissionManage)
		if err != nil {
			return err
		}
		if current != scope {
			return ErrForbidden
		}
		return nil
	})
}

func (s *TargetService) CreateForExecution(ctx context.Context, scope collection.Scope, key string, input TargetInput) (TargetReceipt, error) {
	if ctx == nil || s == nil || scope.Validate() != nil || s.dependencies.ExecutionSources == nil || s.dependencies.ExecutionAuthorization == nil {
		return TargetReceipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	reauthorize := func(ctx context.Context) error {
		if err := s.dependencies.ExecutionAuthorization.AuthorizeExecution(ctx, scope, preparation.PermissionManage); err != nil {
			return ErrForbidden
		}
		return nil
	}
	if err := reauthorize(ctx); err != nil {
		return TargetReceipt{}, err
	}
	return s.create(ctx, scope, key, input, func(ctx context.Context, id string) (preparation.AuthorizedSource, error) {
		return s.dependencies.ExecutionSources.SelectForExecution(ctx, scope, id)
	}, reauthorize)
}

func (s *TargetService) create(ctx context.Context, scope collection.Scope, key string, input TargetInput, selectSource func(context.Context, string) (preparation.AuthorizedSource, error), reauthorize func(context.Context) error) (TargetReceipt, error) {
	if !collection.ValidID(key) || !collection.ValidID(input.SourceID) || !collection.ValidID(input.StoreID) || input.ExpectedRevision < 0 || input.ExpectedRevision >= 1<<63-1 || input.EffectiveVersion == 0 || input.EffectiveVersion > 1<<63-1 || input.ApplyReceiptID != "" && !authidentity.IsBoundedIdentifier(input.ApplyReceiptID) {
		return TargetReceipt{}, ErrInvalid
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > MaxPayloadBytes {
		return TargetReceipt{}, ErrTooLarge
	}
	var copied TargetInput
	if json.Unmarshal(raw, &copied) != nil {
		return TargetReceipt{}, ErrInvalid
	}
	input = copied
	inputHash := collection.Digest(input)
	prior, err := s.dependencies.Records.FindTargetCommand(ctx, scope, key, inputHash)
	if err == nil {
		return prior, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return TargetReceipt{}, err
	}
	selected, err := selectSource(ctx, input.SourceID)
	if err != nil {
		return TargetReceipt{}, err
	}
	selectedScope, source, _, err := selected.Read(ctx)
	if err != nil || selectedScope != scope {
		return TargetReceipt{}, ErrForbidden
	}
	product, err := s.dependencies.Products.ReadEffectiveTargetProduct(ctx, selected, input.EffectiveVersion, input.ApplyReceiptID)
	if err != nil {
		return TargetReceipt{}, err
	}
	if product.Identity.TenantID != scope.OrganizationID || product.Identity.ProductKey != source.Source.ProductKey || product.Version != input.EffectiveVersion || product.PublicationID == "" {
		return TargetReceipt{}, ErrNotReady
	}
	inventoryScope := asset.InventoryScope{TenantID: scope.OrganizationID, ProductKey: source.Source.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: product.Version}
	inventory, err := s.dependencies.Assets.GetApprovedInventory(ctx, inventoryScope)
	if errors.Is(err, asset.ErrApprovedAssetsNotReady) {
		inventory = asset.ApprovedAssetInventory{Scope: inventoryScope, Assets: []asset.ApprovedAsset{}}
	} else if err != nil {
		return TargetReceipt{}, ErrUnavailable
	}
	if inventory.Scope != inventoryScope {
		return TargetReceipt{}, ErrNotReady
	}
	merchant, rules, err := s.dependencies.Rules.ReadTargetRules(ctx, scope, input.StoreID, input.Draft)
	if err != nil {
		return TargetReceipt{}, err
	}
	if merchant.OrganizationID != scope.OrganizationID || merchant.StoreID != input.StoreID || merchant.Site != "shein-us" || merchant.StoreVersion < 1 || merchant.ConnectionRevision < 1 || !merchant.ValidApplication() || string(rules.ApplicationMode) != string(merchant.ApplicationType) || len(merchant.SupplierIdentityHash) != 64 || !time.Now().Before(merchant.ServiceExpiresAt) {
		return TargetReceipt{}, ErrNotReady
	}
	result := goods.BuildOfficial(input.Draft, rules, inventory, nil)
	value := TargetRecord{ID: collection.StableID(scope.OrganizationID, scope.ActorID, "supply-record", key), TargetID: TargetIdentity(scope, source.ID, input.StoreID), Revision: input.ExpectedRevision + 1, Source: source, EffectiveVersion: product.Version, ApplyReceiptID: input.ApplyReceiptID, ProductHash: collection.Digest(product.Snapshot), InventoryHash: collection.Digest(inventory), RulesHash: collection.Digest(rules), Merchant: merchant, Input: input, Result: result, CreatedAt: time.Now().UTC()}
	// Merchant reads can outlast the five-second source proof. Obtain fresh
	// current source authority before committing the same immutable reference.
	selected, err = selectSource(ctx, input.SourceID)
	if err != nil {
		return TargetReceipt{}, err
	}
	freshScope, freshSource, _, err := selected.Read(ctx)
	if err != nil || freshScope != scope || freshSource != source {
		return TargetReceipt{}, ErrConflict
	}
	if err = reauthorize(ctx); err != nil {
		return TargetReceipt{}, err
	}
	return s.dependencies.Records.SaveTarget(ctx, TargetPrepared{scope: scope, key: key, inputHash: inputHash, source: selected, record: value})
}
func (s *TargetService) authorizeTarget(ctx context.Context, purpose string) (collection.Scope, error) {
	scope, err := s.dependencies.Authorizer.Authorize(ctx, purpose)
	if err != nil {
		return collection.Scope{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || scope.Validate() != nil || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || ctx.Err() != nil || !time.Now().Before(identity.TokenExpiresAt) {
		return collection.Scope{}, ErrForbidden
	}
	return scope, nil
}

func (s *TargetService) ReadHead(ctx context.Context, targetID string) (TargetRecord, error) {
	if ctx == nil || s == nil || !collection.ValidID(targetID) {
		return TargetRecord{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorizeTarget(ctx, preparation.PermissionRead)
	if err != nil {
		return TargetRecord{}, err
	}
	return s.dependencies.Records.ReadTargetHead(ctx, scope, targetID)
}
func (s *TargetService) ReadCommand(ctx context.Context, key string) (TargetReceipt, error) {
	if ctx == nil || s == nil || !collection.ValidID(key) {
		return TargetReceipt{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorizeTarget(ctx, preparation.PermissionRead)
	if err != nil {
		return TargetReceipt{}, err
	}
	return s.dependencies.Records.ReadTargetCommand(ctx, scope, key)
}

// OriginalTargetProduct is the explicit unoptimized branch. Non-original
// versions require the Review owner, not an arbitrary Catalog version read.
type OriginalTargetProduct struct{}

func (OriginalTargetProduct) ReadEffectiveTargetProduct(ctx context.Context, selected preparation.AuthorizedSource, version uint64, applyID string) (catalog.PublishedSnapshot, error) {
	_, _, snapshot, err := selected.Read(ctx)
	if err != nil || version != snapshot.Version || applyID != "" {
		return catalog.PublishedSnapshot{}, ErrNotReady
	}
	return snapshot, nil
}

var _ ApprovedAssetReader = (asset.Repository)(nil)
