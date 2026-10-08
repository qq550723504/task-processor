package recordpersistence_test

import (
	"context"
	"errors"
	"time"

	app "task-processor/internal/app/supplychain"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type uploadExecutionRules struct {
	binding storecenter.ProductMerchantBinding
	rules   goods.OfficialRuleSnapshot
}

func (r uploadExecutionRules) ReadTargetRules(context.Context, collection.Scope, string, goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return r.binding, r.rules, nil
}

type uploadExecutionProbe struct{}

func (uploadExecutionProbe) Probe(_ context.Context, a asset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	return goods.OfficialImageObservation{AssetID: a.ID, SourceURL: a.URL, Width: 900, Height: 900, Type: typ, ContentHash: collection.Digest(a.ID), Bytes: 1000, MediaType: "image/jpeg"}, nil
}

// Protocol fixtures only: no merchant credentials, public downloads or paid
// provider traffic. PostgreSQL owners and the actual upload use case execute.
type uploadExecutionMerchant struct {
	binding          storecenter.ProductMerchantBinding
	images, products int
	loseResponse     bool
}

func (m *uploadExecutionMerchant) Binding() storecenter.ProductMerchantBinding { return m.binding }
func (m *uploadExecutionMerchant) PublishPermission(context.Context, string) (model.PublishPermission, error) {
	return model.PublishPermission{Allowed: true}, nil
}
func (m *uploadExecutionMerchant) TransformImage(_ context.Context, input model.TransformImage) (model.TransformedImage, error) {
	m.images++
	return model.TransformedImage{Original: input.OriginalURL, Transformed: "https://img.shein.com/" + collection.Digest(input) + ".jpg", ResponseHash: collection.Digest(input)}, nil
}
func (m *uploadExecutionMerchant) Publish(_ context.Context, input model.PublishProduct) (model.PublishResult, error) {
	m.products++
	if m.loseResponse {
		return model.PublishResult{}, errors.New("fixture lost response")
	}
	return model.PublishResult{SPUName: "fixture-spu", SKCs: []model.PublishedSKC{{SKCName: "fixture-skc", SKUs: []model.PublishedSKU{{SupplierSKU: input.SKCs[0].SKUs[0].SupplierSKU, SKUCode: "fixture-sku"}}}}, ResponseHash: collection.Digest(input)}, nil
}

type uploadExecutionStore struct{ merchant *uploadExecutionMerchant }

func (s uploadExecutionStore) ExecutionMerchant(_ context.Context, scope collection.Scope, id, _ string, expected *storecenter.ProductMerchantBinding) (app.ExecutionMerchant, error) {
	if scope.OrganizationID != s.merchant.binding.OrganizationID || id != s.merchant.binding.StoreID || expected != nil && *expected != s.merchant.binding || !time.Now().Before(s.merchant.binding.ServiceExpiresAt) {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return s.merchant, nil
}
