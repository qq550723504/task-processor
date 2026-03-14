// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"fmt"

	"task-processor/internal/domain/productjson"

	"github.com/sirupsen/logrus"
)

// GenerateJSON 生成产品 JSON
func (g *jsonGenerator) GenerateJSON(ctx context.Context, analysis *productjson.ProductAnalysis, variantGen VariantGenerator) (*productjson.ProductJSON, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	g.logger.Info("generating product JSON")

	productJSON := &productjson.ProductJSON{
		Title:         "Sample Product",
		Category:      []string{"General", "Product"},
		Attributes:    make(map[string]string),
		SellingPoints: []string{"High Quality", "Great Value", "Fast Shipping"},
		SEOKeywords:   []string{"product", "quality", "value"},
		Description:   "This is a high-quality product with great value.",
	}

	// 从分析结果中提取信息
	if analysis.Representation != nil {
		if analysis.Representation.ProductType != "" {
			productJSON.Title = analysis.Representation.ProductType
		}
		if len(analysis.Representation.Attributes) > 0 {
			productJSON.Attributes = analysis.Representation.Attributes
		}
		if len(analysis.Representation.Features) > 0 {
			productJSON.SellingPoints = analysis.Representation.Features
		}
	}

	// 生成产品规格和变体
	if variantGen != nil {
		specs, err := variantGen.GenerateSpecs(ctx, analysis)
		if err != nil {
			logrus.WithError(err).Warn("failed to generate specs")
		} else {
			productJSON.Specifications = specs
		}

		variants, err := variantGen.GenerateVariants(ctx, analysis)
		if err != nil {
			logrus.WithError(err).Warn("failed to generate variants")
		} else {
			productJSON.Variants = variants
		}
	}

	g.logger.Info("product JSON generated successfully")
	return productJSON, nil
}
