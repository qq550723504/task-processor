package httpapi

import (
	"context"
	"gorm.io/gorm"
	"strings"
	supplyapp "task-processor/internal/app/supplychain"
	supplyhttp "task-processor/internal/app/supplychain/httpapi"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
	"time"
)

type PrivateDraftTrialDependencies struct {
	Scope   collection.Scope
	StoreID string
}

func WithPrivateDraftTrial(d PrivateDraftTrialDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.privateDraftTrials++; o.privateDraftTrial = &d }
}

type privateDraftTrialAuthorizer struct {
	base  preparation.Authorizer
	scope collection.Scope
}

func (a privateDraftTrialAuthorizer) Authorize(ctx context.Context, permission string) (collection.Scope, error) {
	scope, err := a.base.Authorize(ctx, permission)
	if err != nil {
		return collection.Scope{}, err
	}
	if permission != preparation.PermissionRead || scope != a.scope {
		return collection.Scope{}, preparation.ErrForbidden
	}
	return scope, nil
}

type privateDraftTrialStore struct {
	scope   collection.Scope
	storeID string
	stores  storecenter.Repository
}
type privateDraftTrialMerchant struct {
	binding storecenter.ProductMerchantBinding
}

func (privateDraftTrialMerchant) Sites(context.Context) ([]model.MainSite, error) {
	return nil, record.ErrUnavailable
}
func (privateDraftTrialMerchant) Warehouses(context.Context) ([]model.Warehouse, error) {
	return nil, record.ErrUnavailable
}
func (privateDraftTrialMerchant) Categories(context.Context) ([]model.Category, error) {
	return nil, record.ErrUnavailable
}
func (privateDraftTrialMerchant) Brands(context.Context) ([]model.Brand, error) {
	return nil, record.ErrUnavailable
}
func (privateDraftTrialMerchant) FillStandards(context.Context, int64) (model.FillStandards, error) {
	return model.FillStandards{}, record.ErrUnavailable
}
func (privateDraftTrialMerchant) Attributes(context.Context, int64) (model.AttributeTemplate, error) {
	return model.AttributeTemplate{}, record.ErrUnavailable
}
func (privateDraftTrialMerchant) LinkedRules(context.Context, model.LinkedRulesRequest) ([]model.LinkedRules, error) {
	return nil, record.ErrUnavailable
}

func (m privateDraftTrialMerchant) Binding() storecenter.ProductMerchantBinding { return m.binding }
func (s privateDraftTrialStore) RulesMerchant(ctx context.Context, scope collection.Scope, id string, expected *storecenter.ProductMerchantBinding) (supplyapp.RulesMerchant, error) {
	if scope != s.scope || id != s.storeID {
		return nil, record.ErrForbidden
	}
	store, err := s.stores.Get(ctx, scope.OrganizationID, id)
	if err != nil || store == nil || store.RecordStatus() != storecenter.RecordStatusActive || store.Platform() != storecenter.PlatformShein || store.ConnectionRef() != "" || !strings.HasPrefix(store.Name(), "离线测试") {
		return nil, record.ErrForbidden
	}
	binding := privateDraftTrialBinding(scope, id, store.Version())
	if expected != nil && *expected != binding {
		return nil, record.ErrConflict
	}
	return privateDraftTrialMerchant{binding: binding}, nil
}
func privateDraftTrialBinding(scope collection.Scope, id string, version int64) storecenter.ProductMerchantBinding {
	return storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: id, Site: "shein-us", StoreVersion: version, ConnectionRevision: 1, ApplicationID: "offline-private-draft-trial", ApplicationType: storecenter.ApplicationSelfOperated, ApplicationRevision: "v1:self_operated", SupplierIdentityHash: collection.Digest([]string{"offline-private-draft-trial", scope.OrganizationID, id}), ServiceExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)}
}
func (s privateDraftTrialStore) validateRecord(ctx context.Context, scope collection.Scope, saved record.TargetRecord) error {
	merchant, err := s.RulesMerchant(ctx, scope, saved.Merchant.StoreID, nil)
	if err != nil {
		return err
	}
	if merchant.Binding() != saved.Merchant {
		return record.ErrForbidden
	}
	return nil
}
func buildPrivateDraftTrialModule(ctx context.Context, productDB, storeDB *gorm.DB, d PrivateDraftTrialDependencies, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, cfg *config.Config) (supplyChainModule, error) {
	var empty supplyChainModule
	if d.Scope.Validate() != nil || !collection.ValidID(d.StoreID) || storeDB == nil || cfg == nil {
		return empty, preparation.ErrUnavailable
	}
	core, err := buildNativeDraftReadCore(ctx, productDB, deps, permissions, func(base preparation.Authorizer) preparation.Authorizer {
		return privateDraftTrialAuthorizer{base, d.Scope}
	})
	if err != nil {
		return empty, err
	}
	stores, err := storecenter.NewMemberScopedStoreRepository(storeDB, currentStoreMemberAuthorizer{authorizer: permissions})
	if err != nil {
		return empty, err
	}
	rules := privateDraftTrialStore{d.Scope, d.StoreID, stores}
	core.app.PublicationStores = rules
	core.app.ValidateDraftRead = rules.validateRecord
	binder := productReviewCapabilityBinder{now: time.Now}
	return supplyChainModule{app: core.app, routes: supplyhttp.PrivateDraftReadRoutes(core.app, binder.Bind)}, nil
}
