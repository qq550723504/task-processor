package sale

import (
	"context"
	"testing"

	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestCallGPTAPI_UsesReducedBatchSizeForLargeVariantSets(t *testing.T) {
	handler := NewSaleAttributeHandler(&batchTestChatClient{
		responses: []string{
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":1,"value":"Red"}]}],"variants":[{"asin":"A01","attributes":{"Color":"Red"}},{"asin":"A02","attributes":{"Color":"Red"}},{"asin":"A03","attributes":{"Color":"Red"}},{"asin":"A04","attributes":{"Color":"Red"}},{"asin":"A05","attributes":{"Color":"Red"}},{"asin":"A06","attributes":{"Color":"Red"}},{"asin":"A07","attributes":{"Color":"Red"}},{"asin":"A08","attributes":{"Color":"Red"}},{"asin":"A09","attributes":{"Color":"Red"}},{"asin":"A10","attributes":{"Color":"Red"}}]}`,
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":2,"value":"Blue"}]}],"variants":[{"asin":"A11","attributes":{"Color":"Blue"}},{"asin":"A12","attributes":{"Color":"Blue"}}]}`,
		},
	})

	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Variations: buildVariationASINs(12),
		},
	}

	request := &sheinattr.GenerationRequest{
		ProductsData:         buildProductVariantData(12),
		VariationData:        buildVariationASINs(12),
		RequiredVariantCount: 12,
	}

	result := handler.callGPTAPI(input, request)

	if got := len(result.Variants); got != 12 {
		t.Fatalf("variant count = %d, want 12", got)
	}
}

func buildVariationASINs(n int) []model.Variation {
	var out []model.Variation
	for i := 1; i <= n; i++ {
		out = append(out, model.Variation{Asin: formatASIN(i)})
	}
	return out
}

func buildProductVariantData(n int) []sheinattr.ProductVariantData {
	var out []sheinattr.ProductVariantData
	for i := 1; i <= n; i++ {
		out = append(out, sheinattr.ProductVariantData{ASIN: formatASIN(i)})
	}
	return out
}

func formatASIN(i int) string {
	if i < 10 {
		return "A0" + string(rune('0'+i))
	}
	return "A" + string(rune('0'+(i/10))) + string(rune('0'+(i%10)))
}
