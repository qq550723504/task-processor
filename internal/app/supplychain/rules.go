package supplychainapp

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
	storeapp "task-processor/internal/app/storecenter"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type RulesMerchant interface {
	Binding() storecenter.ProductMerchantBinding
	Sites(context.Context) ([]model.MainSite, error)
	Categories(context.Context) ([]model.Category, error)
	Brands(context.Context) ([]model.Brand, error)
	FillStandards(context.Context, int64) (model.FillStandards, error)
	Attributes(context.Context, int64) (model.AttributeTemplate, error)
	LinkedRules(context.Context, model.LinkedRulesRequest) ([]model.LinkedRules, error)
}
type RuleStore interface {
	RulesMerchant(context.Context, collection.Scope, string, *storecenter.ProductMerchantBinding) (RulesMerchant, error)
}
type OfficialRuleStore struct {
	Access *storeapp.OfficialProductAccess
}

func (s OfficialRuleStore) RulesMerchant(ctx context.Context, scope collection.Scope, storeID string, expected *storecenter.ProductMerchantBinding) (RulesMerchant, error) {
	if s.Access == nil {
		return nil, record.ErrUnavailable
	}
	return s.Access.Authorize(ctx, storecenter.ProductExecutionSubject{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, Purpose: storecenter.ProductPurposeRules}, storeID, expected)
}

type RuleReader struct{ Stores RuleStore }

func (r RuleReader) ReadTargetRules(ctx context.Context, scope collection.Scope, storeID string, input goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	var snapshot goods.OfficialRuleSnapshot
	if ctx == nil || r.Stores == nil || scope.Validate() != nil || !collection.ValidID(storeID) {
		return storecenter.ProductMerchantBinding{}, snapshot, record.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	merchant, err := r.Stores.RulesMerchant(ctx, scope, storeID, nil)
	if err != nil {
		return storecenter.ProductMerchantBinding{}, snapshot, record.ErrNotReady
	}
	binding := merchant.Binding()
	group, reads := errgroup.WithContext(ctx)
	group.Go(func() error { var err error; snapshot.Sites, err = merchant.Sites(reads); return err })
	group.Go(func() error { var err error; snapshot.Categories, err = merchant.Categories(reads); return err })
	group.Go(func() error { var err error; snapshot.Brands, err = merchant.Brands(reads); return err })
	if group.Wait() != nil {
		return binding, goods.OfficialRuleSnapshot{}, record.ErrUnavailable
	}
	productType, valid := goods.ProductTypeForCategory(snapshot.Categories, input.Product.CategoryID)
	if !valid {
		return binding, snapshot, nil
	} // Preserve the missing-category diagnostic for completion.
	group, reads = errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		snapshot.Fill, err = merchant.FillStandards(reads, input.Product.CategoryID)
		return err
	})
	group.Go(func() error {
		var err error
		snapshot.Attributes, err = merchant.Attributes(reads, productType)
		return err
	})
	if group.Wait() != nil {
		return binding, goods.OfficialRuleSnapshot{}, record.ErrUnavailable
	}
	product := input.Product
	product.ProductTypeID = productType // Canonical leaf owner, never browser-supplied type.
	combinations := goods.LinkedRuleGroups(product)
	chunks := (len(combinations) + 9) / 10
	results := make([][]model.LinkedRules, chunks)
	group, reads = errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i := 0; i < chunks; i++ {
		group.Go(func() error {
			// Fresh short-lived Store handle for each bounded official request.
			current, err := r.Stores.RulesMerchant(reads, scope, storeID, &binding)
			if err != nil {
				return err
			}
			end := min((i+1)*10, len(combinations))
			results[i], err = current.LinkedRules(reads, model.LinkedRulesRequest{Groups: combinations[i*10 : end]})
			return err
		})
	}
	if group.Wait() != nil {
		return binding, goods.OfficialRuleSnapshot{}, record.ErrUnavailable
	}
	for _, result := range results {
		snapshot.Linked = append(snapshot.Linked, result...)
	}
	return binding, snapshot, nil
}

var _ record.TargetRuleReader = RuleReader{}
