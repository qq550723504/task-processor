// Package handlers 提供TEMU平台的SKU构建功能
package handlers

import (
	"fmt"
	"task-processor/internal/common/management/api"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuBuilder SKU构建器
type SkuBuilder struct {
	logger                 *logrus.Entry
	profitRuleClient       api.ProfitRuleAPI
	priceHandler           *PriceHandler
	imageProcessor         *ImageProcessor
	parallelImageProcessor *ParallelImageProcessor
	aiClient               *openai.Client
	specHandler            *SkuSpecHandler
	itemBuilder            *SkuItemBuilder
	skcBuilder             *SkuSkcBuilder
	variantProcessor       *SkuVariantProcessor
	mappingProcessor       *SkuMappingProcessor
	specResolver           *SpecResolverService
}

// NewSkuBuilder 创建新的SKU构建器
func NewSkuBuilder(logger *logrus.Entry, aiClient *openai.Client, profitRuleClient api.ProfitRuleAPI) *SkuBuilder {
	priceHandler := NewPriceHandler(profitRuleClient)
	imageProcessor := NewImageProcessor()
	parallelImageProcessor := NewParallelImageProcessor(3) // 使用3个并发

	specHandler := NewSkuSpecHandler(logger)
	itemBuilder := NewSkuItemBuilder(logger, priceHandler, imageProcessor)
	skcBuilder := NewSkuSkcBuilder(logger, itemBuilder)
	mappingProcessor := NewSkuMappingProcessor(logger, specHandler)

	// 创建SkuBuilder实例（先不初始化variantProcessor和specResolver）
	builder := &SkuBuilder{
		logger:                 logger,
		profitRuleClient:       profitRuleClient,
		priceHandler:           priceHandler,
		imageProcessor:         imageProcessor,
		parallelImageProcessor: parallelImageProcessor,
		aiClient:               aiClient,
		specHandler:            specHandler,
		itemBuilder:            itemBuilder,
		skcBuilder:             skcBuilder,
		mappingProcessor:       mappingProcessor,
	}

	// 创建规格解析服务，传入自己作为API客户端
	builder.specResolver = NewSpecResolverService(builder)

	// 现在创建variantProcessor，传入所有必需的依赖
	builder.variantProcessor = NewSkuVariantProcessor(logger, aiClient, specHandler, skcBuilder, builder.specResolver, itemBuilder)

	return builder
}

// Name 返回处理器名称
func (sb *SkuBuilder) Name() string {
	return "SKU构建器"
}

// HandleTemu 处理任务（强类型上下文）
func (sb *SkuBuilder) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 这里可以根据需要调用具体的构建方法
	// 例如构建默认SKC或处理变体SKC
	_, err := sb.CreateDefaultSkc(temuCtx)
	return err
}

// BuildVariantSkcs 构建变体SKC
func (sb *SkuBuilder) BuildVariantSkcs(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error {
	return sb.variantProcessor.BuildVariantSkcs(temuCtx, variants)
}

// CreateDefaultSkc 创建默认SKC（用于没有变体的产品）
func (sb *SkuBuilder) CreateDefaultSkc(temuCtx *temucontext.TemuTaskContext) (types.Skc, error) {
	return sb.variantProcessor.CreateDefaultSkc(temuCtx)
}

// ProcessSkcItem 处理SKC项目
func (sb *SkuBuilder) ProcessSkcItem(temuCtx *temucontext.TemuTaskContext, skcIndex int) error {
	// 检查TEMU产品数据
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品数据不存在")
	}

	if skcIndex >= len(temuCtx.TemuProduct.SkcList) {
		return fmt.Errorf("SKC索引超出范围: %d >= %d", skcIndex, len(temuCtx.TemuProduct.SkcList))
	}

	skc := &temuCtx.TemuProduct.SkcList[skcIndex]

	// 处理SKC下的每个SKU
	for i := range skc.SkuList {
		if err := sb.itemBuilder.processSkuItem(temuCtx, skcIndex, i); err != nil {
			return fmt.Errorf("处理SKU[%d]失败: %w", i, err)
		}
	}

	return nil
}

// GetTotalSkuCount 获取总SKU数量
func (sb *SkuBuilder) GetTotalSkuCount(skcList []types.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}

// SpecQueryRequest 规格查询请求
type SpecQueryRequest struct {
	GoodsID       string   `json:"goods_id"`
	ChildSpecName string   `json:"child_spec_name"`
	ParentSpecID  string   `json:"parent_spec_id"`
	ExistSpecList []string `json:"exist_spec_list"`
}

// SpecQueryResponse 规格查询响应
type SpecQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		SpecID string `json:"spec_id"`
	} `json:"result"`
}

// QuerySpecID 实现SpecQueryAPI接口，查询规格ID
func (sb *SkuBuilder) QuerySpecID(temuCtx *temucontext.TemuTaskContext, parentSpecID, specName string) (string, error) {
	return sb.querySpecID(temuCtx, parentSpecID, specName)
}

// querySpecID 查询或创建规格ID
func (sb *SkuBuilder) querySpecID(temuCtx *temucontext.TemuTaskContext, parentSpecID, specName string) (string, error) {
	if temuCtx.APIClient == nil {
		return "", fmt.Errorf("API客户端未初始化")
	}

	// 获取goods_id
	goodsID := ""
	if temuCtx.TemuProduct != nil && temuCtx.TemuProduct.GoodsBasic.GoodsID != "" {
		goodsID = temuCtx.TemuProduct.GoodsBasic.GoodsID
	} else {
		return "", fmt.Errorf("goods_id未设置")
	}

	sb.logger.Infof("🔍 查询规格ID: goods_id=%s, parent_spec_id=%s, spec_name=%s",
		goodsID, parentSpecID, specName)

	// 构建请求
	request := SpecQueryRequest{
		GoodsID:       goodsID,
		ChildSpecName: specName,
		ParentSpecID:  parentSpecID,
		ExistSpecList: []string{}, // 可以传入已存在的规格列表
	}

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/spec_query",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": request,
	}

	// 发送请求
	response := &SpecQueryResponse{}
	err := temuCtx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return "", fmt.Errorf("规格查询API调用失败: %w", err)
	}

	if !response.Success {
		// 错误码10000019表示规格不存在或无效，这是数据问题，不应该重试
		if response.ErrorCode == 10000019 {
			sb.logger.Errorf("❌ 规格查询失败: 规格'%s'在TEMU模板中不存在 (parent_spec_id=%s, error_code=%d)",
				specName, parentSpecID, response.ErrorCode)
			sb.logger.Error("💡 可能的原因:")
			sb.logger.Error("   1. AI生成的规格名称与TEMU模板不匹配")
			sb.logger.Error("   2. parent_spec_id不正确")
			sb.logger.Error("   3. 需要在TEMU模板中添加这个规格值")
			return "", fmt.Errorf("NONRETRYABLE: 规格'%s'不存在于TEMU模板中 (error_code=%d)", specName, response.ErrorCode)
		}
		return "", fmt.Errorf("规格查询失败: error_code=%d", response.ErrorCode)
	}

	sb.logger.Infof("✅ 规格查询成功: %s -> %s", specName, response.Result.SpecID)
	return response.Result.SpecID, nil
}
