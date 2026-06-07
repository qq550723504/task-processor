package publish

import (
	"strings"

	sheinctx "task-processor/internal/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

func shouldAllowPrimaryOnlyMultiSKU(ctx *sheinctx.TaskContext, product *sheinproduct.Product) bool {
	if !hasMultiSKUWithinSingleSKC(product) {
		return false
	}
	if ctx == nil || ctx.AttributeTemplates == nil || len(ctx.AttributeTemplates.Data) == 0 {
		return false
	}
	if !hasVaryingSourceDimension(ctx, "size") && !hasVaryingSourceDimension(ctx, "quantity") {
		return false
	}
	return !hasMatchingSecondaryTemplate(ctx.AttributeTemplates.Data[0].AttributeInfos, ctx)
}

func hasMultiSKUWithinSingleSKC(product *sheinproduct.Product) bool {
	if product == nil {
		return false
	}
	for _, skc := range product.SKCList {
		if len(skc.SKUS) > 1 {
			return true
		}
	}
	return false
}

func hasMatchingSecondaryTemplate(attributes []sheinattribute.AttributeInfo, ctx *sheinctx.TaskContext) bool {
	primaryAttrID := 0
	for _, attribute := range attributes {
		if attribute.AttributeLabel == 1 {
			primaryAttrID = attribute.AttributeID
			break
		}
	}
	for _, attribute := range attributes {
		if attribute.AttributeID == primaryAttrID {
			continue
		}
		if attribute.SKCScope != nil && *attribute.SKCScope {
			continue
		}
		name := firstNonEmptyPublish(attribute.AttributeNameEn, attribute.AttributeName)
		if matchesPublishDimension(name, "size") && hasVaryingSourceDimension(ctx, "size") {
			return true
		}
		if matchesPublishDimension(name, "quantity") && hasVaryingSourceDimension(ctx, "quantity") {
			return true
		}
	}
	return false
}

func hasVaryingSourceDimension(ctx *sheinctx.TaskContext, expected string) bool {
	if ctx == nil {
		return false
	}
	if ctx.AmazonProduct != nil {
		values := map[string]struct{}{}
		for _, dimension := range ctx.AmazonProduct.VariationsValues {
			if !matchesPublishDimension(dimension.VariantName, expected) {
				continue
			}
			for _, value := range dimension.Values {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				values[value] = struct{}{}
				if len(values) > 1 {
					return true
				}
			}
		}
	}
	return false
}

func matchesPublishDimension(name, expected string) bool {
	return normalizePublishDimension(name) == normalizePublishDimension(expected)
}

func normalizePublishDimension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "color", "colour", "颜色", "颜色分类":
		return "color"
	case "size", "尺码", "尺寸", "规格":
		return "size"
	case "quantity", "count", "件数", "数量":
		return "quantity"
	case "style", "style type", "款式", "类型":
		return "style"
	default:
		return value
	}
}

func firstNonEmptyPublish(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
