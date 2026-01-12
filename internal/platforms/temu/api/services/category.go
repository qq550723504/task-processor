package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 分类免责声明查询接口
	categoryDisclaimEndpoint = "/mms/marigold/category/query_disclaim"
)

// CategoryAPI 分类API管理器
type CategoryAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewCategoryAPI 创建新的分类API管理器
func NewCategoryAPI(client client.APIClientInterface, logger *logrus.Entry) *CategoryAPI {
	return &CategoryAPI{
		client: client,
		logger: logger,
	}
}

// GetCategoryDisclaimer 获取分类免责声明
func (c *CategoryAPI) GetCategoryDisclaimer(catID int) (*models.CategoryDisclaimResponse, error) {
	c.logger.Infof("获取分类免责声明: CatID=%d", catID)

	if catID == 0 {
		return nil, fmt.Errorf("分类ID不能为空")
	}

	req := &models.CategoryDisclaimRequest{
		CateID: catID,
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"
	headers["x-document-referer"] = "https://seller.temu.com/product-add.html?is_back=1"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     categoryDisclaimEndpoint,
		"headers": headers,
		"body":    req,
	}

	var result models.CategoryDisclaimResponse
	if err := c.client.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("获取分类免责声明失败")
		return nil, fmt.Errorf("获取分类免责声明失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("获取分类免责声明失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取分类免责声明失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Infof("成功获取分类免责声明: %d 条提示", len(result.Result.DisclaimerDTO.PromptList))
	return &result, nil
}

// RecommendCategory 推荐分类
func (c *CategoryAPI) RecommendCategory(request *models.CategoryRecommendRequest) (*models.CategoryRecommendResponse, error) {
	c.logger.Info("开始推荐分类")

	if request == nil {
		return nil, fmt.Errorf("推荐请求不能为空")
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
		"url":     "/mms/marigold/category/recommend",
		"headers": headers,
		"body":    request,
	}

	var result models.CategoryRecommendResponse
	if err := c.client.SendTEMURequest(apiReq, &result); err != nil {
		c.logger.WithError(err).Error("分类推荐请求失败")
		return nil, fmt.Errorf("分类推荐请求失败: %w", err)
	}

	if !result.Success {
		c.logger.Error("分类推荐失败")
		return nil, fmt.Errorf("分类推荐失败")
	}

	c.logger.Infof("成功获取分类推荐: %d 个推荐分类", len(result.Result.CategoryTreeList))
	return &result, nil
}
