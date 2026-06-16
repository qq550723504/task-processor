package sale

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type invalidJSONChatClient struct{}

func (c *invalidJSONChatClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{
				Content: `{"saleAttributes":[{"attrId":27}],"variants":[`,
			},
		}},
	}, nil
}

func (c *invalidJSONChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (c *invalidJSONChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (c *invalidJSONChatClient) GetDefaultModel() string {
	return "test-model"
}

func TestSaleAttributeHandlerCreateChatCompletionRequest_UsesJSONObjectResponseFormat(t *testing.T) {
	t.Parallel()

	handler := NewSaleAttributeHandler(&invalidJSONChatClient{})

	req := handler.createChatCompletionRequest("system", "user", 2)

	if req.ResponseFormat != "json_object" {
		t.Fatalf("ResponseFormat = %q, want %q", req.ResponseFormat, "json_object")
	}
}

func TestProcessSingleBatch_SavesRawResponseOnJSONParseFailure(t *testing.T) {
	t.Parallel()

	handler := NewSaleAttributeHandler(&invalidJSONChatClient{})
	processor := NewSaleAttributeSingleProcessor(handler)
	processor.debugSaver.debugDir = t.TempDir()

	input := &SaleAttributeInput{
		Context: context.Background(),
		Task:    &model.Task{ID: 8213798, ProductID: "B07N14BP26"},
		AmazonProduct: &model.Product{
			Asin: "B07N14BP26",
		},
	}

	result := processor.ProcessSingleBatch(input, &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{{ASIN: "A1"}},
	})
	if len(result.Variants) != 0 {
		t.Fatalf("variants count = %d, want 0 for invalid JSON", len(result.Variants))
	}

	entries, err := os.ReadDir(processor.debugSaver.debugDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("debug files = %d, want 1", len(entries))
	}

	payload, err := os.ReadFile(filepath.Join(processor.debugSaver.debugDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var debugData DebugData
	if err := json.Unmarshal(payload, &debugData); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if debugData.Response == "" {
		t.Fatal("debugData.Response is empty, want raw GPT response")
	}
}
