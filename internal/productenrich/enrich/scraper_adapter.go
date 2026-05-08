package enrich

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/productenrich"
)

type scraper1688 struct {
	processor *alibaba1688.Alibaba1688Processor
}

func NewCrawler1688Adapter(cfg *config.Config) productenrich.WebScraper {
	return &scraper1688{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

func (s *scraper1688) Scrape(_ context.Context, url string) (*productenrich.ScrapedData, error) {
	product, err := s.processor.Process(url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	return build1688ScrapedData(product), nil
}

func build1688ScrapedData(product *alibaba1688model.Product1688) *productenrich.ScrapedData {
	if product == nil {
		return nil
	}

	specs := make(map[string]string, len(product.Specifications))
	for _, sp := range product.Specifications {
		specs[sp.Name] = sp.Value
	}

	return &productenrich.ScrapedData{
		Title:             product.Title,
		Category:          product.Category,
		Description:       build1688Description(product),
		Images:            product.Images,
		Price:             product.MinPrice,
		Specs:             specs,
		VariantDimensions: build1688VariantDimensions(product.VariationsValues),
		Variants:          build1688ScrapedVariants(product),
	}
}

func build1688Description(product *alibaba1688model.Product1688) string {
	if len(product.ProductDetails) == 0 {
		return product.Title
	}
	var sb strings.Builder
	for _, d := range product.ProductDetails {
		if d.Content != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(d.Content)
		}
	}
	if sb.Len() == 0 {
		return product.Title
	}
	return sb.String()
}

func build1688VariantDimensions(values []alibaba1688model.VariationValue) []canonical.ScrapedVariantDimension {
	if len(values) == 0 {
		return nil
	}

	dimensions := make([]canonical.ScrapedVariantDimension, 0, len(values))
	for _, item := range values {
		name := strings.TrimSpace(item.VariantName)
		if name == "" {
			continue
		}

		dimension := canonical.ScrapedVariantDimension{Name: name}
		seen := make(map[string]struct{}, len(item.Values))
		for _, raw := range item.Values {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			dimension.Values = append(dimension.Values, value)
		}
		if len(dimension.Values) == 0 {
			continue
		}
		dimensions = append(dimensions, dimension)
	}

	if len(dimensions) == 0 {
		return nil
	}
	return dimensions
}

func build1688ScrapedVariants(product *alibaba1688model.Product1688) []productenrich.ProductVariant {
	if product == nil || len(product.Variants) == 0 {
		return nil
	}

	variants := make([]productenrich.ProductVariant, 0, len(product.Variants))
	for idx, variant := range product.Variants {
		converted := productenrich.ProductVariant{
			Attributes: convert1688VariantAttributes(variant.Attributes),
			Stock:      variant.Stock,
			Images:     collect1688VariantImages(variant, product.Images),
			IsDefault:  idx == 0,
		}
		converted.SKU = buildScrapedVariantSKU(idx, converted.Attributes)
		if variant.Price > 0 {
			converted.Price = &canonical.PriceInfo{
				Currency:  default1688Currency(product.Currency),
				Amount:    variant.Price,
				CostPrice: variant.Price,
			}
		}
		variants = append(variants, converted)
	}

	if len(variants) == 0 {
		return nil
	}
	return variants
}

func convert1688VariantAttributes(attributes map[string]any) map[string]string {
	if len(attributes) == 0 {
		return map[string]string{}
	}

	converted := make(map[string]string, len(attributes))
	for key, raw := range attributes {
		name := strings.TrimSpace(key)
		value := strings.TrimSpace(stringify1688VariantValue(raw))
		if name == "" || value == "" {
			continue
		}
		converted[name] = value
	}
	return converted
}

func stringify1688VariantValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func collect1688VariantImages(variant alibaba1688model.Variant, fallback []string) []string {
	images := make([]string, 0, 2)
	if image := strings.TrimSpace(variant.Image); image != "" {
		images = append(images, image)
	}
	if len(images) == 0 && len(fallback) > 0 {
		if image := strings.TrimSpace(fallback[0]); image != "" {
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

func default1688Currency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return "CNY"
	}
	return currency
}
