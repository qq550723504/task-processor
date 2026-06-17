package sale

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestBuildBatchProgressFields(t *testing.T) {
	fields := buildBatchProgressFields(2, 5, 10, 4, 20)

	if got := fields["batch"]; got != 2 {
		t.Fatalf("batch = %v, want 2", got)
	}
	if got := fields["total_batches"]; got != 5 {
		t.Fatalf("total_batches = %v, want 5", got)
	}
	if got := fields["batch_size"]; got != 10 {
		t.Fatalf("batch_size = %v, want 10", got)
	}
	if got := fields["batch_variant_count"]; got != 4 {
		t.Fatalf("batch_variant_count = %v, want 4", got)
	}
	if got := fields["processed_variants"]; got != 14 {
		t.Fatalf("processed_variants = %v, want 14", got)
	}
	if got := fields["total_variants"]; got != 20 {
		t.Fatalf("total_variants = %v, want 20", got)
	}
}

type batchTestChatClient struct {
	responses []string
	index     int
}

func (c *batchTestChatClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	content := c.responses[c.index]
	c.index++
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: content},
		}},
	}, nil
}

func (c *batchTestChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (c *batchTestChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (c *batchTestChatClient) GetDefaultModel() string {
	return "test-model"
}

func TestProcessInBatches_MergesSaleAttributeValuesAcrossBatches(t *testing.T) {
	handler := NewSaleAttributeHandler(&batchTestChatClient{
		responses: []string{
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":1,"value":"Red"},{"id":2,"value":"Blue"}]}],"variants":[{"asin":"A1","attributes":{"Color":"Red"}},{"asin":"A2","attributes":{"Color":"Blue"}}]}`,
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":3,"value":"Green"},{"id":4,"value":"Black"}]}],"variants":[{"asin":"A3","attributes":{"Color":"Green"}},{"asin":"A4","attributes":{"Color":"Black"}}]}`,
		},
	})
	processor := NewSaleAttributeBatchProcessor(handler)
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Variations: []model.Variation{{Asin: "A1"}, {Asin: "A2"}, {Asin: "A3"}, {Asin: "A4"}},
			VariationsValues: []model.VariationValue{{
				VariantName: "Color",
				Values:      []string{"Red", "Blue", "Green", "Black"},
			}},
		},
	}

	result := processor.ProcessInBatches(input, &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{
			{ASIN: "A1"}, {ASIN: "A2"}, {ASIN: "A3"}, {ASIN: "A4"},
		},
		VariationData:        make([]model.Variation, 4),
		RequiredVariantCount: 4,
	}, 2)

	if len(result.SaleAttributes) != 1 {
		t.Fatalf("sale attribute count = %d, want 1", len(result.SaleAttributes))
	}
	if got := len(result.SaleAttributes[0].AttrValue); got != 4 {
		t.Fatalf("merged attr value count = %d, want 4", got)
	}
}

func TestProcessInBatches_RejectsCrossBatchASINs(t *testing.T) {
	handler := NewSaleAttributeHandler(&batchTestChatClient{
		responses: []string{
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":1,"value":"Red"}]}],"variants":[{"asin":"A1","attributes":{"Color":"Red"}},{"asin":"A3","attributes":{"Color":"Green"}}]}`,
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":2,"value":"Green"}]}],"variants":[{"asin":"A3","attributes":{"Color":"Green"}},{"asin":"A4","attributes":{"Color":"Black"}}]}`,
		},
	})
	processor := NewSaleAttributeBatchProcessor(handler)
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Variations: []model.Variation{{Asin: "A1"}, {Asin: "A2"}, {Asin: "A3"}, {Asin: "A4"}},
			VariationsValues: []model.VariationValue{{
				VariantName: "Color",
				Values:      []string{"Red", "Blue", "Green", "Black"},
			}},
		},
	}

	result := processor.ProcessInBatches(input, &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{
			{ASIN: "A1"}, {ASIN: "A2"}, {ASIN: "A3"}, {ASIN: "A4"},
		},
		VariationData:        make([]model.Variation, 4),
		RequiredVariantCount: 4,
	}, 2)

	if len(result.Variants) != 0 {
		t.Fatalf("variants = %d, want 0 when batch contains invalid ASINs", len(result.Variants))
	}
}

func TestProcessInBatches_UsesProductsDataAsBatchSourceWhenVariationDataIsLonger(t *testing.T) {
	handler := NewSaleAttributeHandler(&batchTestChatClient{
		responses: []string{
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":1,"value":"Red"}]}],"variants":[{"asin":"A01","attributes":{"Color":"Red"}},{"asin":"A02","attributes":{"Color":"Red"}},{"asin":"A03","attributes":{"Color":"Red"}},{"asin":"A04","attributes":{"Color":"Red"}},{"asin":"A05","attributes":{"Color":"Red"}},{"asin":"A06","attributes":{"Color":"Red"}},{"asin":"A07","attributes":{"Color":"Red"}},{"asin":"A08","attributes":{"Color":"Red"}},{"asin":"A09","attributes":{"Color":"Red"}},{"asin":"A10","attributes":{"Color":"Red"}}]}`,
			`{"saleAttributes":[{"attrId":27,"attrValue":[{"id":2,"value":"Blue"}]}],"variants":[{"asin":"A11","attributes":{"Color":"Blue"}},{"asin":"A12","attributes":{"Color":"Blue"}}]}`,
		},
	})
	client := handler.openaiClient.(*batchTestChatClient)
	processor := NewSaleAttributeBatchProcessor(handler)
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Variations: buildVariationASINs(24),
			VariationsValues: []model.VariationValue{{
				VariantName: "Color",
				Values:      []string{"Red", "Blue"},
			}},
		},
	}

	result := processor.ProcessInBatches(input, &sheinattr.GenerationRequest{
		ProductsData:         buildProductVariantData(12),
		VariationData:        buildVariationASINs(24),
		RequiredVariantCount: 24,
	}, 10)

	if client.index != 2 {
		t.Fatalf("chat client call count = %d, want 2", client.index)
	}
	if got := len(result.Variants); got != 12 {
		t.Fatalf("variants = %d, want 12", got)
	}
}
