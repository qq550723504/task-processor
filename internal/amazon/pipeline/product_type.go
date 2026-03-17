package pipeline

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/amazon/model"
)

type ProductTypeHandler struct {
	*BaseHandler
	services      *model.Services
	marketplaceID string
}

func NewProductTypeHandler(services *model.Services) *ProductTypeHandler {
	return &ProductTypeHandler{
		BaseHandler:   NewBaseHandler("产品类型处理器"),
		services:      services,
		marketplaceID: "ATVPDKIKX0DER",
	}
}

func (h *ProductTypeHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始产品类型识别")

	sourceData := taskContext.Data
	if sourceData == nil {
		return fmt.Errorf("源数据为空")
	}

	if productType := h.getExplicitProductType(sourceData); productType != "" {
		if h.isValidProductType(productType) {
			h.logger.Infof("使用指定的产品类型: %s", productType)
			taskContext.SetResult("product_type", productType)
			return nil
		}
	}

	productType := h.recommendProductType(sourceData)
	if productType != "" {
		h.logger.Infof("推荐产品类型: %s", productType)
		taskContext.SetResult("product_type", productType)
		return nil
	}

	defaultType := "PRODUCT"
	h.logger.Warnf("无法确定产品类型，使用默认类型: %s", defaultType)
	taskContext.SetResult("product_type", defaultType)

	return nil
}

func (h *ProductTypeHandler) getExplicitProductType(sourceData map[string]any) string {
	fields := []string{"product_type", "productType", "category", "type"}

	for _, field := range fields {
		if value, exists := sourceData[field]; exists {
			if str, ok := value.(string); ok && str != "" {
				return strings.ToUpper(str)
			}
		}
	}

	return ""
}

func (h *ProductTypeHandler) isValidProductType(productType string) bool {
	validTypes := map[string]bool{
		"PRODUCT": true, "APPAREL": true, "ELECTRONICS": true,
		"HOME": true, "BEAUTY": true, "SPORTS": true,
		"AUTOMOTIVE": true, "BOOKS": true, "TOYS": true, "LUGGAGE": true,
	}

	return validTypes[strings.ToUpper(productType)]
}

func (h *ProductTypeHandler) recommendProductType(sourceData map[string]any) string {
	keywords := h.extractKeywords(sourceData)

	for _, keyword := range keywords {
		if productType := h.matchKeywordToProductType(keyword); productType != "" {
			return productType
		}
	}

	return ""
}

func (h *ProductTypeHandler) extractKeywords(sourceData map[string]any) []string {
	var keywords []string
	fields := []string{"title", "name", "description", "category"}

	for _, field := range fields {
		if value, exists := sourceData[field]; exists {
			if str, ok := value.(string); ok {
				words := strings.Fields(strings.ToLower(str))
				keywords = append(keywords, words...)
			}
		}
	}

	return keywords
}

func (h *ProductTypeHandler) matchKeywordToProductType(keyword string) string {
	keyword = strings.ToLower(keyword)

	typeMapping := map[string]string{
		"shirt": "APPAREL", "dress": "APPAREL", "clothing": "APPAREL",
		"phone": "ELECTRONICS", "computer": "ELECTRONICS", "laptop": "ELECTRONICS",
		"furniture": "HOME", "kitchen": "HOME",
		"makeup": "BEAUTY", "skincare": "BEAUTY",
		"sports": "SPORTS", "fitness": "SPORTS",
		"car": "AUTOMOTIVE", "auto": "AUTOMOTIVE",
		"book": "BOOKS", "toy": "TOYS",
		"luggage": "LUGGAGE", "bag": "LUGGAGE",
	}

	for key, productType := range typeMapping {
		if strings.Contains(keyword, key) {
			return productType
		}
	}

	return ""
}

