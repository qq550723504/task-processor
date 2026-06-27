package publish

import (
	"context"
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// MarkVariantPublishSuccessHandler marks published and filtered variants.
type MarkVariantPublishSuccessHandler struct {
	logger *logrus.Entry
}

// NewMarkVariantPublishSuccessHandler creates a variant publish result handler.
func NewMarkVariantPublishSuccessHandler() *MarkVariantPublishSuccessHandler {
	return &MarkVariantPublishSuccessHandler{
		logger: logger.GetGlobalLogger("mark_variant_success"),
	}
}

// Name returns the handler name.
func (h *MarkVariantPublishSuccessHandler) Name() string {
	return "开始标记产品发布成功"
}

// Handle marks published variants and filtered variants after publish.
func (h *MarkVariantPublishSuccessHandler) Handle(ctx *shein.TaskContext) error {
	h.logger.Info("start marking published variants")
	if ctx == nil {
		h.logger.Error("task context is nil")
		return fmt.Errorf("task context is nil")
	}

	input, err := buildVariantPublishResultInput(ctx)
	if err != nil {
		return err
	}
	if input.RuntimeRepository == nil {
		h.logger.Warn("runtime repository is nil, skip variant publish marking")
		return nil
	}

	if input.Task != nil && input.SheinResponse != nil {
		if len(input.SheinResponse.Info.PreValidResult) > 0 {
			h.logger.Warnf("found %d validation errors in publish response", len(input.SheinResponse.Info.PreValidResult))
			for _, preValidResult := range input.SheinResponse.Info.PreValidResult {
				h.logger.Warnf("validation error item: %+v", preValidResult)
			}
			return nil
		}

		skus := collectPublishedSupplierSKUs(input)
		h.logger.Infof("start marking %d published SKUs", len(skus))
		successCount := 0
		failCount := 0

		for _, sku := range skus {
			asin := resolveAsinForPublishedSKU(&PublishResultInput{Task: input.Task, AsinSkuMap: input.AsinSkuMap}, sku)
			if asin == "" {
				h.logger.Warnf("missing ASIN for published SKU %s", sku)
				failCount++
				continue
			}
			if err := h.markVariantPublished(input, asin, sku); err != nil {
				h.logger.Errorf("mark variant published failed (ASIN: %s, SKU: %s): %v", asin, sku, err)
				failCount++
			} else {
				successCount++
			}
		}

		h.logger.Infof("variant publish marking done: success=%d fail=%d total=%d", successCount, failCount, len(skus))
	} else {
		h.logger.Warn("task or shein response is unavailable, skip success marking")
	}

	if input.UnfilteredVariants != nil && len(*input.UnfilteredVariants) > 0 {
		for _, variant := range *input.UnfilteredVariants {
			filterInfo := input.GetVariantFilterFn(variant.Asin)
			if filterInfo != nil && filterInfo.FilteredOut {
				if err := h.markVariantFailed(input, variant.Asin, filterInfo.FilterReason); err != nil {
					h.logger.Errorf("mark variant failed failed (ASIN: %s): %v", variant.Asin, err)
				}
			}
		}
	}

	return nil
}

func collectPublishedSupplierSKUs(input *VariantPublishResultInput) []string {
	if input.SheinResponse == nil {
		return nil
	}

	skus := make([]string, 0)
	for _, skc := range input.SheinResponse.Info.SKCList {
		for _, sku := range skc.SKUList {
			skus = append(skus, sku.SupplierSKU)
		}
	}
	return skus
}

func (h *MarkVariantPublishSuccessHandler) markVariantPublished(input *VariantPublishResultInput, asin, sku string) error {
	createReq := buildMappingReq(input.MappingInput, asin, sku, model.TaskStatusPublished)
	id, err := input.RuntimeRepository.CreateRuntimeProductImportMapping(context.Background(), createReq)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"asin":                       asin,
			"sku":                        sku,
			"platform_parent_product_id": createReq.PlatformParentProductID,
			"error":                      err.Error(),
		}).Error("create product import mapping failed")
		return fmt.Errorf("创建产品导入映射关系失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"id":                         id,
		"asin":                       asin,
		"sku":                        sku,
		"platform_parent_product_id": createReq.PlatformParentProductID,
	}).Info("marked variant as published")
	return nil
}

func (h *MarkVariantPublishSuccessHandler) markVariantFailed(input *VariantPublishResultInput, asin, reason string) error {
	createReq := buildMappingReq(input.MappingInput, asin, "", model.TaskStatusCrawlFailed)
	createReq.Remark = &reason

	id, err := input.RuntimeRepository.CreateRuntimeProductImportMapping(context.Background(), createReq)
	if err != nil {
		return fmt.Errorf("创建产品导入映射关系失败: %w", err)
	}

	h.logger.Infof("marked variant as failed (ID: %d, ASIN: %s, Reason: %s)", id, asin, reason)
	return nil
}
