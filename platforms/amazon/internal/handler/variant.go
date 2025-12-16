// Package handler 提供Amazon变体产品处理器
package handler

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/utils"
)

// VariantHandler 变体产品处理器
type VariantHandler struct {
	*BaseHandler
	extractor *utils.VariantExtractor
}

// NewVariantHandler 创建变体处理器
func NewVariantHandler() *VariantHandler {
	return &VariantHandler{
		BaseHandler: NewBaseHandler("变体处理器"),
		extractor:   utils.NewVariantExtractor(),
	}
}

// Execute 处理逻辑
func (h *VariantHandler) Execute(services *model.Services, data map[string]any) error {
	h.logger.Info("开始处理变体产品")

	// 验证服务
	if err := h.ValidateServices(services); err != nil {
		return err
	}

	// 1. 获取原始产品数据
	rawData, exists := data["raw_product_data"]
	if !exists {
		return fmt.Errorf("产品数据不存在")
	}

	productData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 2. 提取变体信息
	variantData, err := h.extractor.ExtractVariants(productData)
	if err != nil {
		return fmt.Errorf("提取变体信息失败: %w", err)
	}

	// 3. 如果不是变体产品，跳过
	if variantData == nil {
		h.logger.Info("这是单品，跳过变体处理")
		h.SetResult(data, "is_variant_product", false)
		return nil
	}

	h.logger.Infof("检测到变体产品，主题: %s", variantData.Theme)
	h.SetResult(data, "is_variant_product", true)
	h.SetResult(data, "variation_theme", variantData.Theme)

	// 4. 获取父产品SKU
	parentSKU, err := h.GetRequiredString(data, "listing_sku")
	if err != nil {
		return fmt.Errorf("获取父产品SKU失败: %w", err)
	}

	// 5. 创建父产品（作为变体容器）
	if err := h.createParentListing(services, data, parentSKU, variantData); err != nil {
		return fmt.Errorf("创建父产品失败: %w", err)
	}

	// 6. 构建子变体
	children, err := h.extractor.BuildVariantChildren(variantData, parentSKU)
	if err != nil {
		return fmt.Errorf("构建子变体失败: %w", err)
	}

	// 7. 创建子变体
	successCount := 0
	for i, child := range children {
		h.logger.Infof("创建子变体 [%d/%d]: %s", i+1, len(children), child.SKU)

		if err := h.createChildListing(services, data, parentSKU, child, variantData.Theme); err != nil {
			h.logger.Warnf("创建子变体失败: %v", err)
			continue
		}

		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("所有子变体创建失败")
	}

	// 8. 保存结果
	h.SetResult(data, "variant_children_count", successCount)
	h.SetResult(data, "variant_children", children)

	h.logger.Infof("变体处理完成，成功创建 %d/%d 个子变体", successCount, len(children))
	return nil
}

// createParentListing 创建父产品
func (h *VariantHandler) createParentListing(
	services *model.Services,
	data map[string]any,
	parentSKU string,
	variantData *utils.VariantData,
) error {
	h.logger.Infof("创建父产品: %s", parentSKU)

	// 获取API客户端
	apiClient, err := h.GetAPIClient(services)
	if err != nil {
		return err
	}

	// 获取映射后的属性
	mappedAttrs, exists := data["mapped_attributes"]
	if !exists {
		return fmt.Errorf("映射后的属性不存在")
	}

	attributes, ok := mappedAttrs.(map[string]any)
	if !ok {
		return fmt.Errorf("属性数据格式错误")
	}

	// 复制属性
	parentAttrs := make(map[string]any)
	for k, v := range attributes {
		parentAttrs[k] = v
	}

	// 添加变体主题
	parentAttrs["variation_theme"] = variantData.Theme

	// 父产品不需要价格和库存
	delete(parentAttrs, "price")
	delete(parentAttrs, "quantity")

	// 构建请求
	req := &api.ListingRequest{
		SKU:          parentSKU,
		ProductType:  h.getProductType(data),
		Requirements: "LISTING",
		Attributes:   parentAttrs,
	}

	// 获取上下文
	ctxValue, exists := data["context"]
	if !exists {
		return fmt.Errorf("上下文不存在")
	}

	ctx, ok := ctxValue.(context.Context)
	if !ok {
		return fmt.Errorf("上下文类型错误")
	}

	// 调用API
	_, err = apiClient.CreateListing(ctx, req)
	if err != nil {
		return fmt.Errorf("创建父产品失败: %w", err)
	}

	h.logger.Info("父产品创建成功")
	return nil
}

// createChildListing 创建子变体
func (h *VariantHandler) createChildListing(
	services *model.Services,
	data map[string]any,
	parentSKU string,
	child utils.VariantChildData,
	theme string,
) error {
	// 获取API客户端
	apiClient, err := h.GetAPIClient(services)
	if err != nil {
		return err
	}

	// 获取基础属性
	mappedAttrs, exists := data["mapped_attributes"]
	if !exists {
		return fmt.Errorf("映射后的属性不存在")
	}

	attributes, ok := mappedAttrs.(map[string]any)
	if !ok {
		return fmt.Errorf("属性数据格式错误")
	}

	// 复制属性（避免修改原始数据）
	childAttrs := make(map[string]any)
	for k, v := range attributes {
		childAttrs[k] = v
	}

	// 添加变体信息
	childAttrs["parent_sku"] = parentSKU
	childAttrs["variation_theme"] = theme

	// 添加变体属性值
	for key, value := range child.VariationData {
		childAttrs[key] = value
	}

	// 添加子变体图片
	if len(child.Images) > 0 {
		childAttrs["main_product_image_locator"] = []map[string]string{
			{"media_location": child.Images[0]},
		}
	}

	// 获取上下文
	ctxValue, exists := data["context"]
	if !exists {
		return fmt.Errorf("上下文不存在")
	}

	ctx, ok := ctxValue.(context.Context)
	if !ok {
		return fmt.Errorf("上下文类型错误")
	}

	// 构建请求
	req := &api.ListingRequest{
		SKU:          child.SKU,
		ProductType:  h.getProductType(data),
		Requirements: "LISTING",
		Attributes:   childAttrs,
	}

	// 调用API创建Listing
	if _, err := apiClient.CreateListing(ctx, req); err != nil {
		return err
	}

	// 设置价格
	if child.Price > 0 {
		priceReq := &api.PriceRequest{
			SKU:      child.SKU,
			Price:    child.Price * 0.14 * 1.30, // 转换并加价
			Currency: "USD",
		}
		if _, err := apiClient.UpdatePrice(ctx, priceReq); err != nil {
			h.logger.Warnf("设置子变体价格失败: %v", err)
		}
	}

	// 设置库存
	if child.Quantity > 0 {
		invReq := &api.InventoryRequest{
			SKU:      child.SKU,
			Quantity: child.Quantity,
		}
		if _, err := apiClient.UpdateInventory(ctx, invReq); err != nil {
			h.logger.Warnf("设置子变体库存失败: %v", err)
		}
	}

	return nil
}

// getProductType 获取产品类型
func (h *VariantHandler) getProductType(data map[string]any) string {
	if productType, exists := data["product_type"]; exists {
		if pt, ok := productType.(string); ok {
			return pt
		}
	}
	return "PRODUCT"
}
