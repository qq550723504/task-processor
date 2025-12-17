package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/shein/api"
	"task-processor/internal/common/shein/api/attribute"
)

// AttributeAPI 属性相关API实现
type AttributeAPI struct {
	*BaseAPIClient
}

// NewAttributeAPI 创建新的属性API实现
func NewAttributeAPI(baseClient *BaseAPIClient) *AttributeAPI {
	return &AttributeAPI{
		BaseAPIClient: baseClient,
	}
}

// GetAttributeTemplates 获取属性模板
func (a *AttributeAPI) GetAttributeTemplates(categoryID int) (*attribute.AttributeTemplateInfo, error) {
	// 参数验证
	if categoryID <= 0 {
		return nil, fmt.Errorf("categoryID必须大于0")
	}

	// 构建URL
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), getAttributeTemplatesEndpoint)

	reqBody := struct {
		CategoryID int     `json:"category_id"`
		ForUpdate  bool    `json:"for_update"`
		SPUName    *string `json:"spu_name"`
	}{
		CategoryID: categoryID,
		ForUpdate:  false,
		SPUName:    nil,
	}

	var result struct {
		api.APIResponse
		Info attribute.AttributeTemplateInfo `json:"info"`
	}

	// 执行请求
	if err := a.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误，包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取属性模板失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

// ValidateCustomAttributeValue 验证自定义属性值
func (a *AttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*attribute.ValidateAttributeResponse, error) {
	// 参数验证
	if attributeID <= 0 {
		return nil, fmt.Errorf("attributeID必须大于0")
	}
	if attributeValue == "" {
		return nil, fmt.Errorf("attributeValue不能为空")
	}
	if categoryID <= 0 {
		return nil, fmt.Errorf("categoryID必须大于0")
	}

	// 构建URL
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), validateAttributeEndpoint)

	requestBody := struct {
		AttributeID    int    `json:"attribute_id"`
		AttributeValue string `json:"attribute_value"`
		CategoryID     int    `json:"category_id"`
		ProductTypeID  *int   `json:"product_type_id"`
		SPUName        string `json:"spu_name"`
	}{
		AttributeID:    attributeID,
		AttributeValue: attributeValue,
		CategoryID:     categoryID,
		ProductTypeID:  nil,
		SPUName:        spuName,
	}

	var result struct {
		api.APIResponse
		Info attribute.ValidateAttributeResponse `json:"info"`
	}
	if err := a.apiRequest(http.MethodPost, url, requestBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		// 其他错误，包装为 APIError
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("验证自定义属性值失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

// AddCustomAttributeValue 添加自定义属性值
func (a *AttributeAPI) AddCustomAttributeValue(req *attribute.AddCustomAttributeValueRequest) (*attribute.AddCustomAttributeValueResponse, error) {
	// 参数验证
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	if req.CategoryID <= 0 {
		return nil, fmt.Errorf("CategoryID必须大于0")
	}
	if len(req.PreAttributeValueList) == 0 {
		return nil, fmt.Errorf("PreAttributeValueList不能为空")
	}

	// 构建URL
	url := fmt.Sprintf("%s%s", a.GetBaseURL(), addAttributeValueEndpoint)

	// 执行请求
	var result attribute.AddCustomAttributeValueResponse
	if err := a.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("添加自定义属性值失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
