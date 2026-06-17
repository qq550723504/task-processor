package sale

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinattr "task-processor/internal/shein/product/attribute"
)

const saleAttributeMaxAttempts = 2

type saleAttributeFailureCategory string

const (
	saleAttributeFailureRequestError    saleAttributeFailureCategory = "request_failed"
	saleAttributeFailureEmptyResponse   saleAttributeFailureCategory = "empty_response"
	saleAttributeFailureTruncated       saleAttributeFailureCategory = "truncated_response"
	saleAttributeFailureInvalidJSON     saleAttributeFailureCategory = "invalid_json"
	saleAttributeFailureVariantMismatch saleAttributeFailureCategory = "variant_count_mismatch"
)

type SaleAttributeSingleProcessor struct {
	handler    *SaleAttributeHandler
	fileSaver  *SaleAttributeFileSaver
	debugSaver *SaleAttributeDebugSaver
}

func NewSaleAttributeSingleProcessor(handler *SaleAttributeHandler) *SaleAttributeSingleProcessor {
	return &SaleAttributeSingleProcessor{handler: handler, fileSaver: NewSaleAttributeFileSaver(), debugSaver: NewSaleAttributeDebugSaver()}
}

func (p *SaleAttributeSingleProcessor) ProcessSingleBatch(input *SaleAttributeInput, request *sheinattr.GenerationRequest) sheinattr.ResultSaleAttribute {
	promptGenerator := NewSaleAttributePromptGenerator()
	systemPrompt := promptGenerator.GenerateSystemPrompt()
	userPrompt := p.handler.buildUserPrompt(input, request)

	taskID := ""
	productID := ""
	if input.Task != nil {
		taskID = fmt.Sprintf("%d", input.Task.ID)
		productID = input.Task.ProductID
	}

	req := p.handler.createChatCompletionRequest(systemPrompt, userPrompt, len(request.ProductsData))
	for attempt := 1; attempt <= saleAttributeMaxAttempts; attempt++ {
		result, category, err, meta := p.processAttempt(input, req, request)
		if err == nil {
			return result
		}

		recoverable := isRecoverableSaleAttributeFailure(category)
		p.logAttemptFailure(taskID, productID, category, attempt, recoverable, err)
		if recoverable && attempt < saleAttributeMaxAttempts {
			continue
		}

		p.persistFailure(taskID, productID, systemPrompt, userPrompt, category, err, meta)
		return sheinattr.ResultSaleAttribute{}
	}

	return sheinattr.ResultSaleAttribute{}
}

func (p *SaleAttributeSingleProcessor) processAttempt(input *SaleAttributeInput, req *openaiclient.ChatCompletionRequest, request *sheinattr.GenerationRequest) (sheinattr.ResultSaleAttribute, saleAttributeFailureCategory, error, DebugMeta) {
	response, err := p.handler.openaiClient.CreateChatCompletion(input.Context, req)
	if err != nil {
		return sheinattr.ResultSaleAttribute{}, saleAttributeFailureRequestError, err, DebugMeta{}
	}
	if len(response.Choices) == 0 {
		emptyErr := fmt.Errorf("GPT response is empty")
		return sheinattr.ResultSaleAttribute{}, saleAttributeFailureEmptyResponse, emptyErr, DebugMeta{
			Model:      response.Model,
			TokensUsed: response.Usage.TotalTokens,
		}
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	meta := DebugMeta{
		Response:     content,
		TokensUsed:   response.Usage.TotalTokens,
		Model:        response.Model,
		FinishReason: response.Choices[0].FinishReason,
	}

	jsonParser := NewSaleAttributeJSONParser()
	if isLikelyTruncatedSaleAttributeResponse(content, response.Choices[0].FinishReason) {
		result := jsonParser.ParseAndValidateJSON(content)
		if len(result.Variants) > 0 && matchesRequiredVariantCount(result, request) {
			return result, "", nil, meta
		}
		return sheinattr.ResultSaleAttribute{}, saleAttributeFailureTruncated, fmt.Errorf("truncated GPT response could not be recovered"), meta
	}

	result := jsonParser.ParseAndValidateJSON(content)
	if len(result.Variants) == 0 {
		return sheinattr.ResultSaleAttribute{}, saleAttributeFailureInvalidJSON, fmt.Errorf("JSON parsing failed or no valid variants"), meta
	}
	if !matchesRequiredVariantCount(result, request) {
		return sheinattr.ResultSaleAttribute{}, saleAttributeFailureVariantMismatch, fmt.Errorf("variant count mismatch: got %d want %d", len(result.Variants), expectedVariantCount(request)), meta
	}
	return result, "", nil, meta
}

func isLikelyTruncatedSaleAttributeResponse(content, finishReason string) bool {
	if finishReason == "length" {
		return true
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	cleaned := strings.TrimSpace(strings.TrimPrefix(trimmed, "```json"))
	cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "```"))
	cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "```"))

	if !strings.HasPrefix(cleaned, "{") {
		return false
	}

	if strings.Count(cleaned, "{") > strings.Count(cleaned, "}") {
		return true
	}
	if strings.Count(cleaned, "[") > strings.Count(cleaned, "]") {
		return true
	}

	lastChar := cleaned[len(cleaned)-1]
	switch lastChar {
	case '{', '[', ':', ',':
		return true
	default:
		return false
	}
}

func withTruncatedDebugMeta(meta DebugMeta) DebugMeta {
	meta.IsTruncated = true
	return meta
}

func isRecoverableSaleAttributeFailure(category saleAttributeFailureCategory) bool {
	switch category {
	case saleAttributeFailureTruncated, saleAttributeFailureInvalidJSON, saleAttributeFailureVariantMismatch:
		return true
	default:
		return false
	}
}

func matchesRequiredVariantCount(result sheinattr.ResultSaleAttribute, request *sheinattr.GenerationRequest) bool {
	expected := expectedVariantCount(request)
	return expected == 0 || len(result.Variants) == expected
}

func expectedVariantCount(request *sheinattr.GenerationRequest) int {
	if request == nil {
		return 0
	}
	if request.RequiredVariantCount > 0 {
		return request.RequiredVariantCount
	}
	return len(request.ProductsData)
}

func (p *SaleAttributeSingleProcessor) logAttemptFailure(taskID, productID string, category saleAttributeFailureCategory, attempt int, recoverable bool, err error) {
	logger.GetGlobalLogger("shein/product").WithFields(map[string]any{
		"task_id":     taskID,
		"product_id":  productID,
		"category":    category,
		"attempt":     attempt,
		"recoverable": recoverable,
	}).Warnf("sale attribute generation failed: %v", err)
}

func (p *SaleAttributeSingleProcessor) persistFailure(taskID, productID, systemPrompt, userPrompt string, category saleAttributeFailureCategory, err error, meta DebugMeta) {
	if category == saleAttributeFailureTruncated {
		_ = p.debugSaver.SaveTruncatedData(taskID, productID+"_fix_failed", systemPrompt, userPrompt, meta)
		_ = p.debugSaver.SaveFailureData(taskID, productID+"_fix_failed", systemPrompt, userPrompt, err, withTruncatedDebugMeta(meta))
		return
	}
	_ = p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, err, meta)
}
