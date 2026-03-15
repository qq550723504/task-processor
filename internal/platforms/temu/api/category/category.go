// Package category 提供TEMU平台分类相关的API和数据结构
package category

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"

	"github.com/sirupsen/logrus"
)

// --- 数据模型 ---

// DisclaimRequest 分类免责声明请求
type DisclaimRequest struct {
	CateID int `json:"cate_id"`
}

// DisclaimResponse 分类免责声明响应
type DisclaimResponse struct {
	Success   bool           `json:"success"`
	ErrorCode int            `json:"error_code"`
	Result    DisclaimResult `json:"result"`
}

// DisclaimResult 分类免责声明结果
type DisclaimResult struct {
	DisclaimerDTO DisclaimerDTO `json:"disclaimer_dto"`
}

// DisclaimerDTO 免责声明数据传输对象
type DisclaimerDTO struct {
	PromptList []string `json:"prompt_list"`
}

// RecommendRequest 分类推荐请求
type RecommendRequest struct {
	GoodsName string `json:"goods_name"`
}

// RecommendResponse 分类推荐响应
type RecommendResponse struct {
	Success bool            `json:"success"`
	Result  RecommendResult `json:"result"`
}

// RecommendResult 分类推荐结果
type RecommendResult struct {
	CategoryTreeList []Category `json:"category_tree_list"`
}

// Category 分类信息
type Category struct {
	CatID        int      `json:"cat_id"`
	Cate1ID      int      `json:"cate1_id"`
	Cate1Name    string   `json:"cate1_name"`
	Cate2ID      int      `json:"cate2_id"`
	Cate2Name    string   `json:"cate2_name"`
	Cate3ID      int      `json:"cate3_id"`
	Cate3Name    string   `json:"cate3_name"`
	Cate4ID      *int     `json:"cate4_id"`
	Cate4Name    *string  `json:"cate4_name"`
	Cate5ID      *int     `json:"cate5_id"`
	Cate5Name    *string  `json:"cate5_name"`
	Cate6ID      *int     `json:"cate6_id"`
	Cate6Name    *string  `json:"cate6_name"`
	Cate7ID      *int     `json:"cate7_id"`
	Cate7Name    *string  `json:"cate7_name"`
	Cate8ID      *int     `json:"cate8_id"`
	Cate8Name    *string  `json:"cate8_name"`
	Cate9ID      *int     `json:"cate9_id"`
	Cate9Name    *string  `json:"cate9_name"`
	Cate10ID     *int     `json:"cate10_id"`
	Cate10Name   *string  `json:"cate10_name"`
	CateNameList []string `json:"cate_name_list"`
	CateType     int      `json:"cate_type"`
	Level        int      `json:"level"`
}

// --- API ---

// API 分类API管理器
type API struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewAPI 创建分类API管理器
func NewAPI(c client.APIClientInterface, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

// GetDisclaimer 获取分类免责声明
func (a *API) GetDisclaimer(catID int) (*DisclaimResponse, error) {
	if catID == 0 {
		return nil, fmt.Errorf("分类ID不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["x-document-referer"] = "https://seller.temu.com/product-add.html?is_back=1"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/category/query_disclaim",
		"headers": headers,
		"body":    &DisclaimRequest{CateID: catID},
	}

	var result DisclaimResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("获取分类免责声明失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("获取分类免责声明失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// Recommend 推荐分类
func (a *API) Recommend(request *RecommendRequest) (*RecommendResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("推荐请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/category/recommend",
		"headers": headers,
		"body":    request,
	}

	var result RecommendResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("分类推荐请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("分类推荐失败")
	}
	return &result, nil
}
