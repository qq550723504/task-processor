package handlers

import (
	"fmt"
	"strings"

	"task-processor/common/pipeline"
)

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

// querySpecID 查询或创建规格ID
func (sb *SkuBuilder) querySpecID(ctx *pipeline.TaskContext, parentSpecID, specName string) (string, error) {
	if ctx.APIClient == nil {
		return "", fmt.Errorf("API客户端未初始化")
	}

	// 获取goods_id
	goodsID := ""
	if ctx.TemuProduct != nil && ctx.TemuProduct.GoodsBasic.GoodsID != "" {
		goodsID = ctx.TemuProduct.GoodsBasic.GoodsID
	} else {
		return "", fmt.Errorf("goods_id未设置")
	}

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
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
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

	return response.Result.SpecID, nil
}

// resolveTemporarySpecIDs 解析临时规格ID为真实规格ID
func (sb *SkuBuilder) resolveTemporarySpecIDs(ctx *pipeline.TaskContext, aiMapping *AISkuMappingResponse) error {
	sb.logger.Info("开始解析临时规格ID")

	for i := range aiMapping.SkuList {
		sku := &aiMapping.SkuList[i]

		for j := range sku.Spec {
			spec := &sku.Spec[j]

			// 检查是否为临时ID
			if strings.HasPrefix(spec.SpecID, "TEMP_") {

				// 调用规格查询API获取真实的spec_id（必须成功）
				realSpecID, err := sb.querySpecID(ctx, spec.ParentSpecID, spec.SpecName)
				if err != nil {
					return fmt.Errorf("规格查询失败 [%s/%s]: %w", spec.ParentSpecName, spec.SpecName, err)
				}

				spec.SpecID = realSpecID
			}
		}

		// 重新生成unique_id（因为spec_id可能已更改）
		if len(sku.Spec) >= 2 {
			sku.UniqueID = fmt.Sprintf("%s_%s", sku.Spec[0].SpecID, sku.Spec[1].SpecID)
		} else if len(sku.Spec) == 1 {
			sku.UniqueID = sku.Spec[0].SpecID
		}

		// 更新color_spec_id和spec_id
		if len(sku.Spec) > 0 {
			// 使用最后一个规格作为主要spec_id
			sku.SpecID = sku.Spec[len(sku.Spec)-1].SpecID
		}
	}

	sb.logger.Info("临时规格ID解析完成")
	return nil
}
