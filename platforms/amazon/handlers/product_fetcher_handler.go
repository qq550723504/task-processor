package handlers

import (
	"fmt"
	"task-processor/common/management/api"
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

// ProductFetcherHandler 产品数据获取处理器
// 从管理系统获取1688产品数据
type ProductFetcherHandler struct {
	rawJsonDataClient api.RawJsonDataAPI
}

// NewProductFetcherHandler 创建产品数据获取处理器
func NewProductFetcherHandler(rawJsonDataClient api.RawJsonDataAPI) *ProductFetcherHandler {
	return &ProductFetcherHandler{
		rawJsonDataClient: rawJsonDataClient,
	}
}

// Name 返回处理器名称
func (h *ProductFetcherHandler) Name() string {
	return "获取1688产品数据"
}

// Handle 处理逻辑
func (h *ProductFetcherHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[ProductFetcher] 开始获取1688产品数据")

	// 检查任务信息
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	// 从管理系统获取1688产品数据
	req := &api.RawJsonDataReqDTO{
		TenantID:   ctx.Task.TenantID,
		Platform:   "1688", // 1688平台
		ProductID:  ctx.Task.ProductID,
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	rawJsonData, err := h.rawJsonDataClient.GetRawJsonData(req)
	if err != nil {
		return fmt.Errorf("获取1688产品数据失败: %w", err)
	}

	if rawJsonData == nil || rawJsonData.RawJSONData == "" {
		return fmt.Errorf("1688产品数据为空: ProductID=%s", ctx.Task.ProductID)
	}

	logrus.Infof("[ProductFetcher] 成功获取1688产品数据: ProductID=%s, 数据长度=%d",
		ctx.Task.ProductID, len(rawJsonData.RawJSONData))

	// 将原始JSON数据保存到上下文
	ctx.SetData("raw_json_data", rawJsonData.RawJSONData)
	ctx.SetData("product_id_1688", ctx.Task.ProductID)

	return nil
}
