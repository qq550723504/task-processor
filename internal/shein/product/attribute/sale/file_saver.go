// Package sale 提供SHEIN平台销售属性文件保存功能
package sale

import (
	"task-processor/internal/core/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

)

// SaleAttributeFileSaver 销售属性文件保存器，负责保存提示词和响应数据到文件
type SaleAttributeFileSaver struct{}

// NewSaleAttributeFileSaver 创建新的销售属性文件保存器
func NewSaleAttributeFileSaver() *SaleAttributeFileSaver {
	return &SaleAttributeFileSaver{}
}

// PromptData 提示词数据结构
type PromptData struct {
	Timestamp    string `json:"timestamp"`
	TaskID       string `json:"task_id"`
	ProductID    string `json:"product_id"`
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
}

// ResponseData 响应数据结构
type ResponseData struct {
	Timestamp    string `json:"timestamp"`
	TaskID       string `json:"task_id"`
	ProductID    string `json:"product_id"`
	RawResponse  string `json:"raw_response"`
	ParsedResult any    `json:"parsed_result"`
	FinishReason string `json:"finish_reason"`
	TokensUsed   int    `json:"tokens_used,omitempty"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// SavePromptData 保存提示词数据到文件
// 参数:
//   - taskID: 任务ID
//   - productID: 产品ID
//   - systemPrompt: 系统提示词
//   - userPrompt: 用户提示词
//
// 返回值:
//   - error: 保存错误信息
func (s *SaleAttributeFileSaver) SavePromptData(taskID, productID, systemPrompt, userPrompt string) error {
	data := PromptData{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		TaskID:       taskID,
		ProductID:    productID,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	filename := fmt.Sprintf("sale_attr_prompt_%s_%s.json", productID, taskID)
	return s.saveToFile(filename, data)
}

// SaveResponseData 保存响应数据到文件
// 参数:
//   - taskID: 任务ID
//   - productID: 产品ID
//   - rawResponse: 原始响应内容
//   - parsedResult: 解析后的结果
//   - finishReason: 完成原因
//   - tokensUsed: 使用的token数量
//   - success: 是否成功
//   - errorMessage: 错误信息
//
// 返回值:
//   - error: 保存错误信息
func (s *SaleAttributeFileSaver) SaveResponseData(taskID, productID, rawResponse string, parsedResult any, finishReason string, tokensUsed int, success bool, errorMessage string) error {
	data := ResponseData{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		TaskID:       taskID,
		ProductID:    productID,
		RawResponse:  rawResponse,
		ParsedResult: parsedResult,
		FinishReason: finishReason,
		TokensUsed:   tokensUsed,
		Success:      success,
		ErrorMessage: errorMessage,
	}

	filename := fmt.Sprintf("sale_attr_response_%s_%s.json", productID, taskID)
	return s.saveToFile(filename, data)
}

// saveToFile 保存数据到文件的通用方法
func (s *SaleAttributeFileSaver) saveToFile(filename string, data any) error {
	// 确保目录存在
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 序列化数据
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join(logsDir, filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logger.GetGlobalLogger("shein/product").Infof("📁 数据已保存到文件: %s", filePath)
	return nil
}


