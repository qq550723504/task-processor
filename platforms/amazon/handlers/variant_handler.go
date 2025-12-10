package handlers

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/utils"

	"github.com/sirupsen/logrus"
)

// VariantHandler 变体产品处理器
type VariantHandler struct {
	apiClient *api.Client
	extractor *utils.VariantExtractor
}

// NewVariantHandler 创建变体处理器
func NewVariantHandler(apiClient *api.Client) *VariantHandler {
	return &VariantHandler{
		apiClient: apiClient,
		extractor: utils.NewVariantExtractor(),
	}
}

// Name 返回处理器名称
func (h *VariantHandler) Name() string {
	return "处理变体产品"
}

// Handle 处理逻辑
func (h *VariantHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[VariantHandler] 开始处理变体产品")

	// 1. 获取原始产品数据
	rawData, exists := ctx.GetData("raw_product_data")
	if !exists {
		return fmt.Errorf("产品数据不存在")
	}

	productData, ok := rawData.(map[string]interface{})
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
		logrus.Info("[VariantHandler] 这是单品，跳过变体处理")
		ctx.SetData("is_variant_product", false)
		return nil
	}

	logrus.Infof("[VariantHandler] 检测到变体产品，主题: %s", variantData.Theme)
	ctx.SetData("is_variant_product", true)
	ctx.SetData("variation_theme", variantData.Theme)

	// 4. 获取父产品SKU
	parentSKU, exists := ctx.GetData("listing_sku")
	if !exists {
		return fmt.Errorf("父产品SKU不存在")
	}

	parentSKUStr, ok := parentSKU.(string)
	if !ok {
		return fmt.Errorf("父产品SKU格式错误")
	}

	// 5. 创建父产品（作为变体容器）
	if err := h.createParentListing(ctx, parentSKUStr, variantData); err != nil {
		return fmt.Errorf("创建父产品失败: %w", err)
	}

	// 6. 构建子变体
	children, err := h.extractor.BuildVariantChildren(variantData, parentSKUStr)
	if err != nil {
		return fmt.Errorf("构建子变体失败: %w", err)
	}

	// 7. 创建子变体
	successCount := 0
	for i, child := range children {
		logrus.Infof("[VariantHandler] 创建子变体 [%d/%d]: %s", i+1, len(children), child.SKU)

		if err := h.createChildListing(ctx, parentSKUStr, child, variantData.Theme); err != nil {
			logrus.Warnf("[VariantHandler] 创建子变体失败: %v", err)
			continue
		}

		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("所有子变体创建失败")
	}

	// 8. 保存结果
	ctx.SetData("variant_children_count", successCount)
	ctx.SetData("variant_children", children)

	logrus.Infof("[VariantHandler] 变体处理完成，成功创建 %d/%d 个子变体",
		successCount, len(children))
	return nil
}

// createParentListing 创建父产品
func (h *VariantHandler) createParentListing(
	ctx *amazon.TaskContext,
	parentSKU string,
	variantData *utils.VariantData,
) error {
	logrus.Infof("[VariantHandler] 创建父产品: %s", parentSKU)

	// 获取映射后的属性
	mappedAttrs, exists := ctx.GetData("mapped_attributes")
	if !exists {
		return fmt.Errorf("映射后的属性不存在")
	}

	attributes, ok := mappedAttrs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("属性数据格式错误")
	}

	// 添加变体主题
	attributes["variation_theme"] = variantData.Theme

	// 父产品不需要价格和库存
	delete(attributes, "price")
	delete(attributes, "quantity")

	// 构建请求
	req := &api.ListingRequest{
		SKU:          parentSKU,
		ProductType:  h.getProductType(ctx),
		Requirements: "LISTING",
		Attributes:   attributes,
	}

	// 调用API
	apiCtx := context.Background()
	_, err := h.apiClient.CreateListing(apiCtx, req)
	if err != nil {
		return fmt.Errorf("创建父产品失败: %w", err)
	}

	logrus.Info("[VariantHandler] 父产品创建成功")
	return nil
}

// createChildListing 创建子变体
func (h *VariantHandler) createChildListing(
	ctx *amazon.TaskContext,
	parentSKU string,
	child utils.VariantChildData,
	theme string,
) error {
	// 获取基础属性
	mappedAttrs, exists := ctx.GetData("mapped_attributes")
	if !exists {
		return fmt.Errorf("映射后的属性不存在")
	}

	attributes, ok := mappedAttrs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("属性数据格式错误")
	}

	// 复制属性（避免修改原始数据）
	childAttrs := make(map[string]interface{})
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

	// 构建请求
	req := &api.ListingRequest{
		SKU:          child.SKU,
		ProductType:  h.getProductType(ctx),
		Requirements: "LISTING",
		Attributes:   childAttrs,
	}

	// 调用API创建Listing
	apiCtx := context.Background()
	if _, err := h.apiClient.CreateListing(apiCtx, req); err != nil {
		return err
	}

	// 设置价格
	if child.Price > 0 {
		priceReq := &api.PriceRequest{
			SKU:      child.SKU,
			Price:    child.Price * 0.14 * 1.30, // 转换并加价
			Currency: "USD",
		}
		if _, err := h.apiClient.UpdatePrice(apiCtx, priceReq); err != nil {
			logrus.Warnf("设置子变体价格失败: %v", err)
		}
	}

	// 设置库存
	if child.Quantity > 0 {
		invReq := &api.InventoryRequest{
			SKU:      child.SKU,
			Quantity: child.Quantity,
		}
		if _, err := h.apiClient.UpdateInventory(apiCtx, invReq); err != nil {
			logrus.Warnf("设置子变体库存失败: %v", err)
		}
	}

	return nil
}

// getProductType 获取产品类型
func (h *VariantHandler) getProductType(ctx *amazon.TaskContext) string {
	if productType, exists := ctx.GetData("product_type"); exists {
		if pt, ok := productType.(string); ok {
			return pt
		}
	}
	return "PRODUCT"
}
