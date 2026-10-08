package supplychainapp

import (
	"context"
	"errors"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type Application struct {
	Preparations          *preparation.Service
	Sources               *preparation.SourceSelector
	Operations            *preparation.OperationService
	Execution             OperationApplication
	Targets               *record.TargetService
	Records               record.TargetRepository
	Products              record.EffectiveTargetProductReader
	Rules                 record.TargetRuleReader
	Assets                record.ApprovedAssetReader
	Approvals             *asset.SourceApprovalService
	Authorization         preparation.Authorizer
	PublicationReceipts   submission.OfficialReceiptRepository
	PublicationStores     RuleStore
	AuthorizeOptimization func(context.Context, preparation.OperationInput) error
	OptimizationOptions   func(context.Context, collection.Query) (OptimizationOptions, error)
	StageProjection       ReviewProjection
}
type SourceImageView struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type SourceDetail struct {
	Source  preparation.SourceItem  `json:"source"`
	Product catalog.ProductSnapshot `json:"product"`
	Images  []SourceImageView       `json:"images"`
}

func (a *Application) Source(ctx context.Context, id string) (SourceDetail, error) {
	selected, err := a.Sources.Select(ctx, id)
	if err != nil {
		return SourceDetail{}, err
	}
	_, source, product, err := selected.Read(ctx)
	if err != nil {
		return SourceDetail{}, err
	}
	images := []SourceImageView{}
	for _, image := range SourceImages(product) {
		images = append(images, SourceImageView{image.ID, image.URL, image.Width, image.Height})
	}
	return SourceDetail{Source: source, Product: product.Snapshot, Images: images}, nil
}

type TargetRules struct {
	Merchant storecenter.ProductMerchantBinding `json:"merchant"`
	Rules    goods.OfficialRuleSnapshot         `json:"rules"`
}

func (a *Application) QueryRules(ctx context.Context, in record.TargetInput) (TargetRules, error) {
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return TargetRules{}, err
	}
	selected, err := a.Sources.Select(ctx, in.SourceID)
	if err != nil {
		return TargetRules{}, err
	}
	owner, _, _, err := selected.Read(ctx)
	if err != nil || owner != scope {
		return TargetRules{}, preparation.ErrForbidden
	}
	if _, err = a.Products.ReadEffectiveTargetProduct(ctx, selected, in.EffectiveVersion, in.ApplyReceiptID); err != nil {
		return TargetRules{}, err
	}
	merchant, rules, err := a.Rules.ReadTargetRules(ctx, scope, in.StoreID, in.Draft)
	return TargetRules{merchant, rules}, err
}
func (a *Application) ReadTarget(ctx context.Context, sourceID, storeID string) (record.TargetRecord, error) {
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return record.TargetRecord{}, err
	}
	selected, err := a.Sources.Select(ctx, sourceID)
	if err != nil {
		return record.TargetRecord{}, err
	}
	owner, _, _, err := selected.Read(ctx)
	if err != nil || owner != scope {
		return record.TargetRecord{}, preparation.ErrForbidden
	}
	if !collection.ValidID(storeID) {
		return record.TargetRecord{}, record.ErrInvalid
	}
	return a.Targets.ReadHead(ctx, record.TargetIdentity(scope, sourceID, storeID))
}
func (a *Application) ReadRecord(ctx context.Context, id string) (record.TargetRecord, error) {
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return record.TargetRecord{}, err
	}
	if !collection.ValidID(id) {
		return record.TargetRecord{}, record.ErrInvalid
	}
	value, err := a.Records.ReadTargetRecord(ctx, scope, id)
	if err != nil {
		return record.TargetRecord{}, err
	}
	if _, err = a.Sources.Select(ctx, value.Source.ID); err != nil {
		return record.TargetRecord{}, err
	}
	return value, nil
}
func (a *Application) Inventory(ctx context.Context, input asset.SourceSelectionRequest) (asset.ApprovedAssetInventory, error) {
	reader := SourceImageReader{Sources: a.Sources, Products: a.Products, Authorization: a.Authorization}
	selection, err := reader.ReadSourceSelection(ctx, input)
	if err != nil {
		return asset.ApprovedAssetInventory{}, err
	}
	scope := asset.InventoryScope{TenantID: selection.TenantID, ProductKey: selection.ProductKey, TargetPlatform: selection.TargetPlatform, SourceSnapshotVersion: selection.EffectiveCatalogVersion}
	inventory, err := a.Assets.GetApprovedInventory(ctx, scope)
	if errors.Is(err, asset.ErrApprovedAssetsNotReady) {
		return asset.ApprovedAssetInventory{Scope: scope, Assets: []asset.ApprovedAsset{}}, nil
	}
	return inventory, err
}
func (a *Application) OperationByKey(ctx context.Context, key string) (preparation.Operation, error) {
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return preparation.Operation{}, err
	}
	if !collection.ValidID(key) {
		return preparation.Operation{}, preparation.ErrInvalid
	}
	return a.Operations.Read(ctx, preparation.OperationCommandID(scope, key))
}
