// Package sale 提供SHEIN平台的销售属性单批处理功能
package sale

import (
	"task-processor/internal/core/logger"
	"fmt"
	"strings"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"

)

// SaleAttributeSingleProcessor 销售属性单批处理器，负责处理单个批次的GPT API调用
type SaleAttributeSingleProcessor struct {
	handler    *SaleAttributeHandler
	fileSaver  *SaleAttributeFileSaver
	debugSaver *SaleAttributeDebugSaver
}

// NewSaleAttributeSingleProcessor 创建新的销售属性单批处理器
// 参数:
//   - handler: 销售属性处理器实例
//
// 返回值:
//   - *SaleAttributeSingleProcessor: 单批处理器实例
func NewSaleAttributeSingleProcessor(handler *SaleAttributeHandler) *SaleAttributeSingleProcessor {
	return &SaleAttributeSingleProcessor{
		handler:    handler,
		fileSaver:  NewSaleAttributeFileSaver(),
		debugSaver: NewSaleAttributeDebugSaver(),
	}
}

// ProcessSingleBatch 单批次调用GPT API
// 参数:
//   - ctx: 任务上下文
//   - request: 生成请求
//
// 返回值:
//   - ResultSaleAttribute: 销售属性结果
func (p *SaleAttributeSingleProcessor) ProcessSingleBatch(ctx *sheinctx.TaskContext, request *sheinattr.GenerationRequest) sheinattr.ResultSaleAttribute {
	promptGenerator := NewSaleAttributePromptGenerator()
	systemPrompt := promptGenerator.GenerateSystemPrompt()

	// 构建用户提示词
	userPrompt := p.handler.buildUserPrompt(ctx, request)

	// 获取任务和产品ID用于调试数据保存
	taskID := ""
	productID := ""
	if ctx.Task != nil {
		taskID = fmt.Sprintf("%d", ctx.Task.ID)
		productID = ctx.Task.ProductID
	}

	req := p.handler.createChatCompletionRequest(systemPrompt, userPrompt, len(request.ProductsData))

	response, err := p.handler.openaiClient.CreateChatCompletion(ctx.Context, req)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 调用GPT API失败: %v", err)

		// 保存失败的调试数据
		if saveErr := p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, err); saveErr != nil {
			logger.GetGlobalLogger("shein/product").Errorf("⚠️ 保存调试数据失败: %v", saveErr)
		}

		return sheinattr.ResultSaleAttribute{}
	}

	if len(response.Choices) == 0 {
		logger.GetGlobalLogger("shein/product").Error("❌ GPT API响应为空")

		// 保存空响应的调试数据
		emptyErr := fmt.Errorf("GPT API响应为空")
		if saveErr := p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, emptyErr); saveErr != nil {
			logger.GetGlobalLogger("shein/product").Errorf("⚠️ 保存调试数据失败: %v", saveErr)
		}

		return sheinattr.ResultSaleAttribute{}
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	tokensUsed := response.Usage.TotalTokens
	isTruncated := response.Choices[0].FinishReason == "length"

	// 检查响应是否被截断
	if isTruncated {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ GPT响应被截断（达到token限制），响应长度: %d字符", len(content))
		logger.GetGlobalLogger("shein/product").Warn("⚠️ 尝试修复并解析部分JSON...")

		// 保存截断响应的调试数据
		if saveErr := p.debugSaver.SaveTruncatedData(taskID, productID, systemPrompt, userPrompt,
			content, response.Model, tokensUsed); saveErr != nil {
			logger.GetGlobalLogger("shein/product").Errorf("⚠️ 保存截断响应调试数据失败: %v", saveErr)
		}

		// 尝试修复被截断的JSON
		jsonParser := NewSaleAttributeJSONParser()
		result := jsonParser.ParseAndValidateJSON(content)

		if len(result.Variants) > 0 {
			logger.GetGlobalLogger("shein/product").Infof("✅ 成功从截断的响应中解析出%d个变体", len(result.Variants))
			return result
		}

		logger.GetGlobalLogger("shein/product").Error("❌ 无法从截断的响应中解析有效数据，建议增加MaxTokens")

		// 保存修复失败的调试数据
		fixErr := fmt.Errorf("无法从截断的响应中解析有效数据，建议增加MaxTokens")
		if saveErr := p.debugSaver.SaveFailureData(taskID, productID+"_fix_failed", systemPrompt, userPrompt, fixErr); saveErr != nil {
			logger.GetGlobalLogger("shein/product").Errorf("⚠️ 保存修复失败调试数据失败: %v", saveErr)
		}

		return sheinattr.ResultSaleAttribute{}
	}

	// 清理和验证JSON
	jsonParser := NewSaleAttributeJSONParser()
	result := jsonParser.ParseAndValidateJSON(content)

	// 如果解析失败，保存调试数据
	if len(result.Variants) == 0 {
		parseErr := fmt.Errorf("JSON解析失败或无有效变体数据")
		if saveErr := p.debugSaver.SaveFailureData(taskID, productID, systemPrompt, userPrompt, parseErr); saveErr != nil {
			logger.GetGlobalLogger("shein/product").Errorf("⚠️ 保存解析失败调试数据失败: %v", saveErr)
		}
	}

	return result
}


