package sale

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	sheinattr "task-processor/internal/shein/product/attribute"
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
	response, err := p.handler.openaiClient.CreateChatCompletion(input.Context, req)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("GPT request failed: %v", err)
		_ = p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, "", err)
		return sheinattr.ResultSaleAttribute{}
	}
	if len(response.Choices) == 0 {
		emptyErr := fmt.Errorf("GPT response is empty")
		_ = p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, "", emptyErr)
		return sheinattr.ResultSaleAttribute{}
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if response.Choices[0].FinishReason == "length" {
		jsonParser := NewSaleAttributeJSONParser()
		result := jsonParser.ParseAndValidateJSON(content)
		if len(result.Variants) > 0 {
			return result
		}
		fixErr := fmt.Errorf("truncated GPT response could not be recovered")
		_ = p.debugSaver.SaveFailureData(taskID, productID+"_fix_failed", systemPrompt, userPrompt, content, fixErr)
		return sheinattr.ResultSaleAttribute{}
	}

	jsonParser := NewSaleAttributeJSONParser()
	result := jsonParser.ParseAndValidateJSON(content)
	if len(result.Variants) == 0 {
		parseErr := fmt.Errorf("JSON parsing failed or no valid variants")
		_ = p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, content, parseErr)
	}
	return result
}
