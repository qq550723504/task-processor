// Package modules 提供SHEIN平台的销售属性调试数据保存功能
package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// SaleAttributeDebugSaver 销售属性调试数据保存器
type SaleAttributeDebugSaver struct {
	debugDir string
}

// NewSaleAttributeDebugSaver 创建调试数据保存器
func NewSaleAttributeDebugSaver() *SaleAttributeDebugSaver {
	return &SaleAttributeDebugSaver{
		debugDir: "logs/debug/sale_attribute",
	}
}

// DebugData 调试数据结构
type DebugData struct {
	Timestamp    time.Time `json:"timestamp"`
	TaskID       string    `json:"task_id"`
	ProductID    string    `json:"product_id"`
	SystemPrompt string    `json:"system_prompt"`
	UserPrompt   string    `json:"user_prompt"`
	Response     string    `json:"response,omitempty"`
	Error        string    `json:"error,omitempty"`
	TokensUsed   int       `json:"tokens_used,omitempty"`
	Model        string    `json:"model,omitempty"`
	IsTruncated  bool      `json:"is_truncated"`
}

// SaveFailureData 保存失败的调试数据
func (s *SaleAttributeDebugSaver) SaveFailureData(taskID, productID, systemPrompt, userPrompt string, err error) error {
	return s.saveDebugData(taskID, productID, systemPrompt, userPrompt, "", err, 0, "", false)
}

// SaveTruncatedData 保存被截断的调试数据
func (s *SaleAttributeDebugSaver) SaveTruncatedData(taskID, productID, systemPrompt, userPrompt, response, model string, tokensUsed int) error {
	return s.saveDebugData(taskID, productID, systemPrompt, userPrompt, response, nil, tokensUsed, model, true)
}

// saveDebugData 保存调试数据
func (s *SaleAttributeDebugSaver) saveDebugData(taskID, productID, systemPrompt, userPrompt, response string, err error, tokensUsed int, model string, isTruncated bool) error {
	// 确保目录存在
	if createErr := os.MkdirAll(s.debugDir, 0755); createErr != nil {
		return fmt.Errorf("创建调试目录失败: %w", createErr)
	}

	// 构建调试数据
	debugData := DebugData{
		Timestamp:    time.Now(),
		TaskID:       taskID,
		ProductID:    productID,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Response:     response,
		TokensUsed:   tokensUsed,
		Model:        model,
		IsTruncated:  isTruncated,
	}

	if err != nil {
		debugData.Error = err.Error()
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	status := "failure"
	if err == nil && isTruncated {
		status = "truncated"
	}

	filename := fmt.Sprintf("debug_%s_%s_%s_%s.json", status, taskID, productID, timestamp)
	filepath := filepath.Join(s.debugDir, filename)

	// 保存到JSON文件
	jsonData, err := json.MarshalIndent(debugData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化调试数据失败: %w", err)
	}

	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入调试文件失败: %w", err)
	}

	logrus.Infof("💾 调试数据已保存: %s", filepath)
	return nil
}
