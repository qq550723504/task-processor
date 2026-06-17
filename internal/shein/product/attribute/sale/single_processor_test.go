package sale

import (
	"context"
	"encoding/json"
	"errors"
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

type truncatedJSONChatClient struct{}

func (c *truncatedJSONChatClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return &openaiclient.ChatCompletionResponse{
		Model: "test-model",
		Usage: openaiclient.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Choices: []openaiclient.ChatCompletionChoice{{
			FinishReason: "stop",
			Message: openaiclient.ChatCompletionMessage{
				Content: "```json\n{\"saleAttributes\":[],\"variants\":[{\"asin\":\"A1\",\"quantity\":1",
			},
		}},
	}, nil
}

func (c *truncatedJSONChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (c *truncatedJSONChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (c *truncatedJSONChatClient) GetDefaultModel() string {
	return "test-model"
}

type sequentialChatClient struct {
	responses []*openaiclient.ChatCompletionResponse
	errors    []error
	callCount int
}

func (c *sequentialChatClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	idx := c.callCount
	c.callCount++

	if idx < len(c.errors) && c.errors[idx] != nil {
		return nil, c.errors[idx]
	}
	if idx < len(c.responses) && c.responses[idx] != nil {
		return c.responses[idx], nil
	}
	return &openaiclient.ChatCompletionResponse{}, nil
}

func (c *sequentialChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (c *sequentialChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (c *sequentialChatClient) GetDefaultModel() string {
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

func TestProcessSingleBatch_MarksLikelyTruncatedResponseEvenWhenFinishReasonIsStop(t *testing.T) {
	t.Parallel()

	handler := NewSaleAttributeHandler(&truncatedJSONChatClient{})
	processor := NewSaleAttributeSingleProcessor(handler)
	processor.debugSaver.debugDir = t.TempDir()

	input := &SaleAttributeInput{
		Context: context.Background(),
		Task:    &model.Task{ID: 8213798, ProductID: "B07N14BP26"},
	}

	result := processor.ProcessSingleBatch(input, &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{{ASIN: "A1"}},
	})
	if len(result.Variants) != 0 {
		t.Fatalf("variants count = %d, want 0 for truncated JSON", len(result.Variants))
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
	if !debugData.IsTruncated {
		t.Fatal("debugData.IsTruncated = false, want true for obviously truncated response")
	}
	if debugData.FinishReason != "stop" {
		t.Fatalf("debugData.FinishReason = %q, want %q", debugData.FinishReason, "stop")
	}
	if debugData.Model != "test-model" {
		t.Fatalf("debugData.Model = %q, want %q", debugData.Model, "test-model")
	}
	if debugData.TokensUsed != 30 {
		t.Fatalf("debugData.TokensUsed = %d, want 30", debugData.TokensUsed)
	}
}

func TestProcessSingleBatch_RetriesOnceOnInvalidJSON(t *testing.T) {
	t.Parallel()

	client := &sequentialChatClient{
		responses: []*openaiclient.ChatCompletionResponse{
			{
				Model: "test-model",
				Choices: []openaiclient.ChatCompletionChoice{{
					FinishReason: "stop",
					Message: openaiclient.ChatCompletionMessage{
						Content: `{"saleAttributes":[{"attrId":27}],"variants":[`,
					},
				}},
			},
			{
				Model: "test-model",
				Choices: []openaiclient.ChatCompletionChoice{{
					FinishReason: "stop",
					Message: openaiclient.ChatCompletionMessage{
						Content: `{"saleAttributes":[{"attrId":27,"attrValue":[]}],"variants":[{"asin":"A1","quantity":1}]}`,
					},
				}},
			},
		},
	}

	handler := NewSaleAttributeHandler(client)
	processor := NewSaleAttributeSingleProcessor(handler)
	processor.debugSaver.debugDir = t.TempDir()

	input := &SaleAttributeInput{
		Context: context.Background(),
		Task:    &model.Task{ID: 8213798, ProductID: "B07N14BP26"},
	}

	result := processor.ProcessSingleBatch(input, &sheinattr.GenerationRequest{
		ProductsData:         []sheinattr.ProductVariantData{{ASIN: "A1"}},
		RequiredVariantCount: 1,
	})

	if client.callCount != 2 {
		t.Fatalf("CreateChatCompletion call count = %d, want 2", client.callCount)
	}
	if len(result.Variants) != 1 {
		t.Fatalf("variants count = %d, want 1", len(result.Variants))
	}
}

func TestProcessSingleBatch_RetriesOnceOnVariantCountMismatch(t *testing.T) {
	t.Parallel()

	client := &sequentialChatClient{
		responses: []*openaiclient.ChatCompletionResponse{
			{
				Model: "test-model",
				Choices: []openaiclient.ChatCompletionChoice{{
					FinishReason: "stop",
					Message: openaiclient.ChatCompletionMessage{
						Content: `{"saleAttributes":[{"attrId":27,"attrValue":[]}],"variants":[]}`,
					},
				}},
			},
			{
				Model: "test-model",
				Choices: []openaiclient.ChatCompletionChoice{{
					FinishReason: "stop",
					Message: openaiclient.ChatCompletionMessage{
						Content: `{"saleAttributes":[{"attrId":27,"attrValue":[]}],"variants":[{"asin":"A1","quantity":1}]}`,
					},
				}},
			},
		},
	}

	handler := NewSaleAttributeHandler(client)
	processor := NewSaleAttributeSingleProcessor(handler)
	processor.debugSaver.debugDir = t.TempDir()

	input := &SaleAttributeInput{
		Context: context.Background(),
		Task:    &model.Task{ID: 8213798, ProductID: "B07N14BP26"},
	}

	result := processor.ProcessSingleBatch(input, &sheinattr.GenerationRequest{
		ProductsData:         []sheinattr.ProductVariantData{{ASIN: "A1"}},
		RequiredVariantCount: 1,
	})

	if client.callCount != 2 {
		t.Fatalf("CreateChatCompletion call count = %d, want 2", client.callCount)
	}
	if len(result.Variants) != 1 {
		t.Fatalf("variants count = %d, want 1", len(result.Variants))
	}
}

func TestProcessSingleBatch_DoesNotRetryOnRequestError(t *testing.T) {
	t.Parallel()

	client := &sequentialChatClient{
		errors: []error{errors.New("gateway timeout")},
	}

	handler := NewSaleAttributeHandler(client)
	processor := NewSaleAttributeSingleProcessor(handler)
	processor.debugSaver.debugDir = t.TempDir()

	input := &SaleAttributeInput{
		Context: context.Background(),
		Task:    &model.Task{ID: 8213798, ProductID: "B07N14BP26"},
	}

	result := processor.ProcessSingleBatch(input, &sheinattr.GenerationRequest{
		ProductsData:         []sheinattr.ProductVariantData{{ASIN: "A1"}},
		RequiredVariantCount: 1,
	})

	if client.callCount != 1 {
		t.Fatalf("CreateChatCompletion call count = %d, want 1", client.callCount)
	}
	if len(result.Variants) != 0 {
		t.Fatalf("variants count = %d, want 0", len(result.Variants))
	}
}
