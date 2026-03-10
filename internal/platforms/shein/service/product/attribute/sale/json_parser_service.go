// Package modules 提供SHEIN平台的销售属性JSON解析功能
package sale

import (
	"encoding/json"
	"regexp"
	"strings"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// SaleAttributeJSONParser 销售属性JSON解析器，负责解析和修复GPT API返回的JSON数据
type SaleAttributeJSONParser struct{}

// NewSaleAttributeJSONParser 创建新的销售属性JSON解析器
// 返回值:
//   - *SaleAttributeJSONParser: JSON解析器实例
func NewSaleAttributeJSONParser() *SaleAttributeJSONParser {
	return &SaleAttributeJSONParser{}
}

// ParseAndValidateJSON 解析和验证JSON
// 参数:
//   - content: 要解析的JSON内容
//
// 返回值:
//   - ResultSaleAttribute: 解析后的销售属性结果
func (p *SaleAttributeJSONParser) ParseAndValidateJSON(content string) model.ResultSaleAttribute {
	logrus.Infof("📝 开始解析AI响应，长度: %d 字符", len(content))

	// 清理JSON格式
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		logrus.Debug("清理了markdown代码块标记")
	}
	content = strings.TrimSpace(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		logrus.Warn("⚠️ JSON格式无效，尝试修复...")

		// 尝试修复常见的JSON问题
		fixedContent := p.fixCommonJsonIssues(content)
		if !json.Valid([]byte(fixedContent)) {
			logrus.Error("❌ JSON修复失败，无法解析")
			logrus.Debugf("原始内容前500字符: %s", content[:min(500, len(content))])
			return model.ResultSaleAttribute{}
		}
		logrus.Info("✅ JSON修复成功")
		content = fixedContent
	} else {
		logrus.Debug("✅ JSON格式有效")
	}

	var saleAttributeData model.ResultSaleAttribute
	if err := jsonutil.UnmarshalBytes([]byte(content), &saleAttributeData, "JSON解析失败"); err != nil {
		logrus.Errorf("❌ JSON解析失败: %v", err)
		logrus.Debugf("内容前500字符: %s", content[:min(500, len(content))])
		return model.ResultSaleAttribute{}
	}

	logrus.Infof("✅ 成功解析AI响应 - 销售属性: %d 个, 变体: %d 个",
		len(saleAttributeData.SaleAttributes), len(saleAttributeData.Variants))

	return saleAttributeData
}

// fixCommonJsonIssues 修复常见的JSON问题
func (p *SaleAttributeJSONParser) fixCommonJsonIssues(content string) string {
	original := content
	logrus.Infof("🔧 开始修复JSON，原始长度: %d", len(content))

	// 1. 移除尾部的无效内容（在最后一个有效结构后的说明文字）
	content = p.removeTrailingExplanation(content)

	// 2. 修复被截断的JSON对象（移除最后一个不完整的对象）
	content = p.removeIncompleteLastObject(content)

	// 3. 修复尾部缺失的中括号
	openBrackets := strings.Count(content, "[")
	closeBrackets := strings.Count(content, "]")
	if openBrackets > closeBrackets {
		missing := openBrackets - closeBrackets
		logrus.Infof("🔧 修复缺失的%d个中括号", missing)
		// 在最后一个 } 之前添加缺失的 ]
		lastBraceIndex := strings.LastIndex(content, "}")
		if lastBraceIndex > 0 {
			content = content[:lastBraceIndex] + strings.Repeat("]", missing) + content[lastBraceIndex:]
		} else {
			content = content + strings.Repeat("]", missing)
		}
	}

	// 4. 修复尾部缺失的大括号
	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces > closeBraces {
		missing := openBraces - closeBraces
		logrus.Infof("🔧 修复缺失的%d个大括号", missing)
		content = content + strings.Repeat("}", missing)
	}

	// 5. 修复双引号问题
	content = strings.ReplaceAll(content, `\"`, `"`)

	// 6. 确保JSON以大括号开始和结束
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		logrus.Warnf("JSON不以{开头，添加开头大括号")
		content = "{" + content
	}
	if !strings.HasSuffix(content, "}") {
		logrus.Warnf("JSON不以}结尾，添加结尾大括号")
		content = content + "}"
	}

	if content != original {
		logrus.Infof("✅ JSON修复完成，原始长度: %d, 修复后长度: %d", len(original), len(content))
	} else {
		logrus.Debug("JSON无需修复")
	}

	return content
}

// removeIncompleteLastObject 移除被截断的最后一个不完整对象
func (p *SaleAttributeJSONParser) removeIncompleteLastObject(content string) string {
	// 查找variants数组的最后一个逗号位置
	// 如果JSON被截断，最后一个对象可能不完整，需要移除

	// 查找"variants"数组
	variantsIndex := strings.Index(content, `"variants"`)
	if variantsIndex == -1 {
		return content
	}

	// 从variants位置开始查找
	afterVariants := content[variantsIndex:]

	// 查找最后一个完整的对象结束位置（},）
	lastCompleteObjectPattern := regexp.MustCompile(`\},\s*\{[^}]*$`)
	if match := lastCompleteObjectPattern.FindStringIndex(afterVariants); match != nil {
		// 找到了不完整的最后一个对象，截断到最后一个完整对象
		cutPosition := variantsIndex + match[0] + 1 // +1保留}
		logrus.Infof("🔧 检测到不完整的最后一个对象，截断位置: %d", cutPosition)
		content = content[:cutPosition] + "\n]}"
	}

	return content
}

// removeTrailingExplanation 移除JSON后的说明文字
func (p *SaleAttributeJSONParser) removeTrailingExplanation(content string) string {
	// 查找可能的说明文字开始位置
	// 通常说明文字以 "### " 或 "**" 或换行符+中文开始
	patterns := []string{
		"\n###",
		"\n**",
		"\n\n1.",
		"\n\n-",
		"```\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			// 检查这个位置之前是否有完整的JSON结构
			beforePattern := content[:idx]
			if p.looksLikeCompleteJson(beforePattern) {
				logrus.Infof("检测到说明文字开始于位置%d，移除后续内容", idx)
				return beforePattern
			}
		}
	}

	return content
}

// looksLikeCompleteJson 检查内容是否看起来像完整的JSON
func (p *SaleAttributeJSONParser) looksLikeCompleteJson(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")
}
