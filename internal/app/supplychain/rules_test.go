package supplychainapp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type rulesFixture struct {
	mu      sync.Mutex
	binding storecenter.ProductMerchantBinding
	groups  []model.LinkedRuleGroup
	changed bool
}

func (s *rulesFixture) RulesMerchant(_ context.Context, scope collection.Scope, storeID string, expected *storecenter.ProductMerchantBinding) (RulesMerchant, error) {
	if scope.OrganizationID != s.binding.OrganizationID || storeID != s.binding.StoreID || expected != nil && (*expected != s.binding || s.changed) {
		return nil, record.ErrForbidden
	}
	return s, nil
}
func (s *rulesFixture) Binding() storecenter.ProductMerchantBinding { return s.binding }
func (s *rulesFixture) Sites(context.Context) ([]model.MainSite, error) {
	return []model.MainSite{}, nil
}
func (s *rulesFixture) Warehouses(context.Context) ([]model.Warehouse, error) {
	return []model.Warehouse{}, nil
}
func (s *rulesFixture) Categories(context.Context) ([]model.Category, error) {
	leaf := true
	return []model.Category{{ID: 123, ProductTypeID: 456, Leaf: &leaf}}, nil
}
func (s *rulesFixture) Brands(context.Context) ([]model.Brand, error) { return []model.Brand{}, nil }
func (s *rulesFixture) FillStandards(context.Context, int64) (model.FillStandards, error) {
	return model.FillStandards{}, nil
}
func (s *rulesFixture) Attributes(context.Context, int64) (model.AttributeTemplate, error) {
	return model.AttributeTemplate{ProductTypeID: 456}, nil
}
func (s *rulesFixture) LinkedRules(_ context.Context, input model.LinkedRulesRequest) ([]model.LinkedRules, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(input.Groups) > 10 {
		return nil, record.ErrInvalid
	}
	output := []model.LinkedRules{}
	for _, group := range input.Groups {
		s.groups = append(s.groups, group)
		output = append(output, model.LinkedRules{GroupID: group.ID, Attributes: []model.LinkedAttributeRule{}})
	}
	return output, nil
}
func TestMerchantRuleReaderCapturesAllCombinationsWithExactStoreBinding(t *testing.T) {
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	storeID := uuid.NewString()
	fixture := &rulesFixture{binding: storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 2, ApplicationRevision: "v1:self_operated", ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, SupplierIdentityHash: collection.Digest("merchant"), ServiceExpiresAt: time.Now().Add(time.Hour)}}
	input := goods.OfficialDraftInput{Product: model.PublishProduct{CategoryID: 123, ProductTypeID: 999}}
	for i := 0; i < 25; i++ {
		input.Product.SKCs = append(input.Product.SKCs, model.ProductSKC{SKUs: []model.ProductSKU{{}}})
	}
	reader := RuleReader{Stores: fixture}
	binding, snapshot, err := reader.ReadTargetRules(context.Background(), scope, storeID, input)
	require.NoError(t, err)
	require.Equal(t, fixture.binding, binding)
	require.Len(t, snapshot.Linked, 26)
	require.Len(t, fixture.groups, 26)
	for _, group := range fixture.groups {
		require.EqualValues(t, 456, group.ProductTypeID)
		require.EqualValues(t, 123, group.CategoryID)
	}
	fixture.changed = true
	_, _, err = reader.ReadTargetRules(context.Background(), scope, storeID, input)
	require.ErrorIs(t, err, record.ErrUnavailable, "changed merchant binding never mixes rules from another connection")
}
