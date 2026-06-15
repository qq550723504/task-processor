package sale

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

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
