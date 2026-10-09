package goods

import (
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"testing"
)

func TestPublishedMembershipRequiresAllSKUIdentifiersInTheirOriginalGroups(t *testing.T) {
	input := model.PublishProduct{SKCs: []model.ProductSKC{{SKUs: []model.ProductSKU{{SupplierSKU: "sku-a"}}}, {SKUs: []model.ProductSKU{{SupplierSKU: "sku-b"}}}}}
	result := model.PublishResult{SPUName: "spu-a", SKCs: []model.PublishedSKC{{SKCName: "skc-b", SKUs: []model.PublishedSKU{{SupplierSKU: "sku-b", SKUCode: "code-b"}}}, {SKCName: "skc-a", SKUs: []model.PublishedSKU{{SupplierSKU: "sku-a", SKUCode: "code-a"}}}}}
	require.True(t, CorrelatesPublishedMembership(input, result))
	result.SKCs[0].SKUs[0].SKUCode = "code-a"
	require.False(t, CorrelatesPublishedMembership(input, result))
	result.SKCs[0].SKUs[0].SKUCode = "code-b"
	result.SKCs[0].SKUs = append(result.SKCs[0].SKUs, result.SKCs[1].SKUs...)
	result.SKCs[1].SKUs = nil
	require.False(t, CorrelatesPublishedMembership(input, result), "merged provider groups cannot count as a full matched publication")
}
