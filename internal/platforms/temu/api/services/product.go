// Package services 提供TEMU平台产品API功能
package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// ProductAPI 产品API管理器
type ProductAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewProductAPI 创建新的产品API管理器
func NewProductAPI(client client.APIClientInterface, logger *logrus.Entry) *ProductAPI {
	return &ProductAPI{
		client: client,
		logger: logger,
	}
}

// ListProducts 获取产品列表
// 参考接口: POST https://seller.temu.com/mms/marigold/sku/v2/search
func (p *ProductAPI) ListProducts(pageNo, pageSize int) (*models.ProductListResponse, error) {
	request := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/v2/search",
		"headers": map[string]string{
			"content-type":       "application/json;charset=UTF-8",
			"x-document-referer": "https://seller.temu.com/",
		},
		"body": models.ProductListRequest{
			PageSize:              pageSize,
			PageNo:                pageNo,
			OrderType:             0,            // 降序
			OrderField:            "gmt_create", // 按创建时间排序
			EnableBatchSearchText: true,
			SkuSearchType:         2, // 全部
		},
	}

	var result models.ProductListResponse
	authManager := client.NewAuthManager(p.logger)
	if err := authManager.SendRequestWithAuth(p.client, request, &result); err != nil {
		return nil, fmt.Errorf("调用产品列表 API 失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 返回错误: error_code=%d", result.ErrorCode)
	}

	p.logger.WithFields(logrus.Fields{
		"page_no":   pageNo,
		"page_size": pageSize,
		"total":     result.Result.Total,
		"count":     len(result.Result.GoodsList),
	}).Info("成功获取 TEMU 产品列表")

	return &result, nil
}

// ListOnShelfProducts 获取已上架的产品列表
func (p *ProductAPI) ListOnShelfProducts(pageNo, pageSize int) ([]models.TemuProductResponse, error) {
	response, err := p.ListProducts(pageNo, pageSize)
	if err != nil {
		return nil, err
	}

	// 过滤出已上架的产品 (status4_vo == 3)
	var onShelfProducts []models.TemuProductResponse
	for _, product := range response.Result.GoodsList {
		if product.Status4Vo == 3 {
			onShelfProducts = append(onShelfProducts, product)
		}
	}

	p.logger.WithFields(logrus.Fields{
		"total":    len(response.Result.GoodsList),
		"on_shelf": len(onShelfProducts),
	}).Info("过滤已上架产品")

	return onShelfProducts, nil
}

// GetProduct 获取单个产品详情
func (p *ProductAPI) GetProduct(goodsID string) (*models.TemuProductResponse, error) {
	// 先获取产品列表，然后查找指定的产品
	// TEMU 可能有专门的产品详情接口，这里暂时使用列表接口
	response, err := p.ListProducts(1, 100)
	if err != nil {
		return nil, err
	}

	for _, product := range response.Result.GoodsList {
		if product.GoodsID == goodsID {
			return &product, nil
		}
	}

	return nil, fmt.Errorf("未找到产品: %s", goodsID)
}

// SaveProduct 保存产品到草稿箱
func (p *ProductAPI) SaveProduct(request *models.ProductSaveRequest) (*models.ProductSaveResponse, error) {
	p.logger.Info("开始保存产品到TEMU草稿箱")

	if request == nil {
		return nil, fmt.Errorf("保存请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     "/mms/marigold/edit/commit/save",
		"headers": headers,
		"body":    request,
	}

	var result models.ProductSaveResponse
	if err := p.client.SendTEMURequest(apiReq, &result); err != nil {
		p.logger.WithError(err).Error("发送产品保存请求失败")
		return nil, fmt.Errorf("发送产品保存请求失败: %w", err)
	}

	if !result.Success {
		p.logger.Errorf("产品保存失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
		return nil, fmt.Errorf("产品保存失败: errorCode=%d, message=%s", result.ErrorCode, result.Message)
	}

	p.logger.Infof("产品保存成功: ListingCommitID=%s",
		func() string {
			if result.Result != nil {
				return result.Result.ListingCommitID
			}
			return "未返回"
		}())

	return &result, nil
}
